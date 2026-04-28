use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

pub struct ServiceTask {
    name: &'static str,
    interval: Duration,
    job: Box<dyn FnMut() -> Result<(), String> + Send + 'static>,
}

impl ServiceTask {
    pub fn periodic<F>(name: &'static str, interval: Duration, job: F) -> Self
    where
        F: FnMut() -> Result<(), String> + Send + 'static,
    {
        Self {
            name,
            interval,
            job: Box::new(job),
        }
    }
}

pub struct ServiceTaskRunner {
    stop_signal: Arc<AtomicBool>,
    handles: Vec<JoinHandle<()>>,
}

impl ServiceTaskRunner {
    pub fn start(
        tasks: Vec<ServiceTask>,
        log: Arc<dyn Fn(String) + Send + Sync + 'static>,
    ) -> Self {
        let stop_signal = Arc::new(AtomicBool::new(false));
        let handles = tasks
            .into_iter()
            .map(|task| start_task(task, stop_signal.clone(), log.clone()))
            .collect();
        Self {
            stop_signal,
            handles,
        }
    }

    pub fn stop_and_join(self) {
        self.stop_signal.store(true, Ordering::SeqCst);
        for handle in self.handles {
            let _ = handle.join();
        }
    }
}

fn start_task(
    mut task: ServiceTask,
    stop_signal: Arc<AtomicBool>,
    log: Arc<dyn Fn(String) + Send + Sync + 'static>,
) -> JoinHandle<()> {
    thread::spawn(move || {
        log(format!(
            "task {} start interval_secs={}",
            task.name,
            task.interval.as_secs()
        ));
        let mut failure_count = 0_u32;
        while !stop_signal.load(Ordering::SeqCst) {
            match (task.job)() {
                Ok(()) => {
                    if failure_count > 0 {
                        log(format!("task {} recovered", task.name));
                    }
                    failure_count = 0;
                }
                Err(err) => {
                    failure_count = failure_count.saturating_add(1);
                    if failure_count == 1 || failure_count % 12 == 0 {
                        log(format!(
                            "task {} skipped count={} error={}",
                            task.name, failure_count, err
                        ));
                    }
                }
            }
            sleep_with_stop(&stop_signal, task.interval);
        }
        log(format!("task {} stopped", task.name));
    })
}

fn sleep_with_stop(stop_signal: &AtomicBool, duration: Duration) {
    let deadline = Instant::now() + duration;
    while !stop_signal.load(Ordering::SeqCst) && Instant::now() < deadline {
        let remaining = deadline.saturating_duration_since(Instant::now());
        thread::sleep(remaining.min(Duration::from_millis(200)));
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::sync::atomic::AtomicUsize;
    use std::sync::mpsc;

    #[test]
    fn runner_executes_periodic_jobs_until_stopped() {
        let runs = Arc::new(AtomicUsize::new(0));
        let job_runs = runs.clone();
        let (tx, rx) = mpsc::channel();
        let runner = ServiceTaskRunner::start(
            vec![ServiceTask::periodic(
                "test-periodic",
                Duration::from_millis(5),
                move || {
                    let count = job_runs.fetch_add(1, Ordering::SeqCst) + 1;
                    let _ = tx.send(count);
                    Ok(())
                },
            )],
            Arc::new(|_| {}),
        );

        let mut observed = 0;
        while observed < 2 {
            observed = rx
                .recv_timeout(Duration::from_secs(1))
                .expect("periodic task should run");
        }
        runner.stop_and_join();

        assert!(runs.load(Ordering::SeqCst) >= 2);
    }

    #[test]
    fn runner_logs_failures_without_stopping_task() {
        let attempts = Arc::new(AtomicUsize::new(0));
        let job_attempts = attempts.clone();
        let (attempt_tx, attempt_rx) = mpsc::channel();
        let (log_tx, log_rx) = mpsc::channel();
        let runner = ServiceTaskRunner::start(
            vec![ServiceTask::periodic(
                "test-failure",
                Duration::from_millis(5),
                move || {
                    let count = job_attempts.fetch_add(1, Ordering::SeqCst) + 1;
                    let _ = attempt_tx.send(count);
                    Err("not ready".to_string())
                },
            )],
            Arc::new(move |message| {
                let _ = log_tx.send(message);
            }),
        );

        let mut observed = 0;
        while observed < 2 {
            observed = attempt_rx
                .recv_timeout(Duration::from_secs(1))
                .expect("failing task should keep retrying");
        }
        runner.stop_and_join();

        assert!(attempts.load(Ordering::SeqCst) >= 2);
        let logs = log_rx.try_iter().collect::<Vec<_>>().join("\n");
        assert!(logs.contains("task test-failure skipped"));
        assert!(logs.contains("not ready"));
    }
}
