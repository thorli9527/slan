create table if not exists login_failures (
  key varchar(255) primary key,
  failed_count integer not null,
  first_failed_at timestamptz not null,
  last_failed_at timestamptz not null,
  blocked_until timestamptz
);

create index if not exists ix_login_failures_blocked_until on login_failures(blocked_until);
