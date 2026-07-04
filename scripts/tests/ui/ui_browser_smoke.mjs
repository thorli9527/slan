#!/usr/bin/env node
import { createRequire } from 'node:module';
import { spawn } from 'node:child_process';
import { mkdir, mkdtemp, writeFile, rm } from 'node:fs/promises';
import { existsSync } from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const scriptDir = path.dirname(fileURLToPath(import.meta.url));
let rootDir = scriptDir;
while (!existsSync(path.join(rootDir, '.git')) && rootDir !== path.dirname(rootDir)) {
  rootDir = path.dirname(rootDir);
}
if (!existsSync(path.join(rootDir, '.git'))) {
  throw new Error(`repo root not found from ${scriptDir}`);
}
const WebSocket = require(path.join(rootDir, 'server/opt-ui/node_modules/ws'));

const webBase = process.env.SLAN_UI_SMOKE_WEB_BASE || 'http://47.245.40.231:24200';
const opsBase = process.env.SLAN_UI_SMOKE_OPS_BASE || 'http://47.245.40.231:24201';
const chromePath = process.env.CHROME_BIN || '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const outDir = process.env.SLAN_UI_SMOKE_OUT_DIR || path.join(rootDir, '.tmp/ui-browser-smoke');
const runID = Date.now();

if (!existsSync(chromePath)) {
  throw new Error(`Chrome not found: ${chromePath}`);
}

await mkdir(outDir, { recursive: true });

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function waitFor(fn, timeoutMs, label) {
  const started = Date.now();
  let lastError;
  while (Date.now() - started < timeoutMs) {
    try {
      const value = await fn();
      if (value) return value;
    } catch (error) {
      lastError = error;
    }
    await sleep(150);
  }
  throw new Error(`timeout waiting for ${label}${lastError ? `: ${lastError.message}` : ''}`);
}

async function fetchJSON(url, options) {
  const response = await fetch(url, options);
  if (!response.ok) {
    throw new Error(`${url} returned HTTP ${response.status}`);
  }
  return response.json();
}

class CDPPage {
  constructor(wsURL, label) {
    this.wsURL = wsURL;
    this.label = label;
    this.nextID = 1;
    this.pending = new Map();
    this.consoleErrors = [];
    this.pageErrors = [];
    this.networkErrors = [];
  }

  async connect() {
    this.ws = new WebSocket(this.wsURL);
    this.ws.on('message', (raw) => this.onMessage(JSON.parse(raw.toString())));
    await new Promise((resolve, reject) => {
      this.ws.once('open', resolve);
      this.ws.once('error', reject);
    });
    await this.send('Page.enable');
    await this.send('Runtime.enable');
    await this.send('Log.enable');
    await this.send('Network.enable');
  }

  onMessage(message) {
    if (message.id && this.pending.has(message.id)) {
      const { resolve, reject } = this.pending.get(message.id);
      this.pending.delete(message.id);
      if (message.error) reject(new Error(message.error.message || JSON.stringify(message.error)));
      else resolve(message.result);
      return;
    }
    if (message.method === 'Runtime.consoleAPICalled' && ['error', 'assert'].includes(message.params.type)) {
      const text = message.params.args?.map((arg) => arg.value ?? arg.description ?? '').join(' ');
      this.consoleErrors.push(text || message.params.type);
    }
    if (message.method === 'Runtime.exceptionThrown') {
      this.pageErrors.push(message.params.exceptionDetails?.text || 'Runtime exception');
    }
    if (message.method === 'Log.entryAdded' && ['error'].includes(message.params.entry.level)) {
      this.consoleErrors.push(message.params.entry.text || 'log error');
    }
    if (message.method === 'Network.responseReceived') {
      const { response } = message.params;
      if (response.status >= 400 && !response.url.includes('favicon')) {
        this.networkErrors.push(`${response.status} ${response.url}`);
      }
    }
    if (message.method === 'Network.loadingFailed') {
      const blocked = message.params.blockedReason || '';
      const errorText = message.params.errorText || blocked;
      if (errorText && errorText !== 'net::ERR_ABORTED') {
        this.networkErrors.push(`${errorText}`);
      }
    }
  }

  send(method, params = {}) {
    const id = this.nextID++;
    this.ws.send(JSON.stringify({ id, method, params }));
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      setTimeout(() => {
        if (this.pending.has(id)) {
          this.pending.delete(id);
          reject(new Error(`CDP timeout: ${method}`));
        }
      }, 15000);
    });
  }

  async navigate(url) {
    this.consoleErrors = [];
    this.pageErrors = [];
    this.networkErrors = [];
    await this.send('Page.navigate', { url });
    await this.waitLoad();
  }

  async waitLoad() {
    await waitFor(async () => {
      const state = await this.eval('document.readyState');
      return state === 'complete' || state === 'interactive';
    }, 20000, `${this.label} document load`);
    await sleep(500);
  }

  async eval(expression) {
    const result = await this.send('Runtime.evaluate', {
      expression,
      awaitPromise: true,
      returnByValue: true,
      userGesture: true,
    });
    if (result.exceptionDetails) {
      const details = result.exceptionDetails;
      const message = details.exception?.description || details.exception?.value || details.text || 'Runtime.evaluate exception';
      throw new Error(message);
    }
    return result.result?.value;
  }

  async installHelpers() {
    await this.eval(`(() => {
      window.__uiSmoke = {
        text(el) { return (el?.innerText || el?.textContent || '').replace(/\\s+/g, ' ').trim(); },
        visible(el) {
          if (!el || el.disabled) return false;
          const style = getComputedStyle(el);
          const box = el.getBoundingClientRect();
          return style.visibility !== 'hidden' && style.display !== 'none' && box.width > 0 && box.height > 0;
        },
        findClickable(text, selector = 'button,a,[role="button"]') {
          const exact = [...document.querySelectorAll(selector)].find((el) => this.visible(el) && this.text(el) === text);
          if (exact) return exact;
          return [...document.querySelectorAll(selector)].find((el) => {
            if (!this.visible(el)) return false;
            const content = this.text(el);
            const title = (el.getAttribute('title') || '').trim();
            const aria = (el.getAttribute('aria-label') || '').trim();
            return content.includes(text) || title.includes(text) || aria.includes(text);
          });
        },
        clickText(text, selector) {
          const el = this.findClickable(text, selector);
          if (!el) throw new Error('click target not found: ' + text);
          el.scrollIntoView({ block: 'center', inline: 'center' });
          el.click();
          return this.text(el);
        },
        clickRowButton(rowText, buttonText) {
          const row = [...document.querySelectorAll('tr')].find((el) => this.visible(el) && this.text(el).includes(rowText));
          if (!row) throw new Error('row not found: ' + rowText);
          const button = [...row.querySelectorAll('button,a,[role="button"]')].find((el) => {
            if (!this.visible(el)) return false;
            const content = this.text(el);
            const title = (el.getAttribute('title') || '').trim();
            const aria = (el.getAttribute('aria-label') || '').trim();
            return content.includes(buttonText) || title.includes(buttonText) || aria.includes(buttonText);
          });
          if (!button) throw new Error('row button not found: ' + rowText + ' / ' + buttonText);
          button.scrollIntoView({ block: 'center', inline: 'center' });
          button.click();
          return this.text(button) || button.getAttribute('title') || button.getAttribute('aria-label') || '';
        },
        clickModalButton(text) {
          const button = [...document.querySelectorAll('.modal-backdrop button')].find((el) => this.visible(el) && this.text(el).includes(text));
          if (!button) throw new Error('modal button not found: ' + text);
          button.scrollIntoView({ block: 'center', inline: 'center' });
          button.click();
          return this.text(button);
        },
        targetBox(text, selector) {
          const el = this.findClickable(text, selector);
          if (!el) throw new Error('click target not found: ' + text);
          el.scrollIntoView({ block: 'center', inline: 'center' });
          const box = el.getBoundingClientRect();
          const x = box.left + box.width / 2;
          const y = box.top + box.height / 2;
          const top = document.elementFromPoint(x, y);
          return { text: this.text(el), tag: el.tagName, className: el.className || '', topText: this.text(top), topTag: top?.tagName || '', x, y };
        },
        setValue(el, value) {
          if (!el) throw new Error('input not found');
          el.scrollIntoView({ block: 'center', inline: 'center' });
          el.focus();
          el.value = value;
          el.dispatchEvent(new Event('input', { bubbles: true }));
          el.dispatchEvent(new Event('change', { bubbles: true }));
        },
        setInput(index, value) {
          const inputs = [...document.querySelectorAll('input')].filter((el) => this.visible(el));
          this.setValue(inputs[index], value);
        },
        heading() {
          return this.text(document.querySelector('main h1, h1'));
        },
        h2s() {
          return [...document.querySelectorAll('h2')].filter((el) => this.visible(el)).map((el) => this.text(el));
        },
        closeOverlays() {
          let count = 0;
          for (let i = 0; i < 8; i++) {
            const close = [...document.querySelectorAll('.modal-backdrop button, .tag-popover button')]
              .find((el) => this.visible(el) && ['x', '×', '关闭', '取消', '完成', '确认'].some((text) => this.text(el).includes(text) || el.title === text));
            if (!close) break;
            close.click();
            count++;
          }
          return count;
        },
        activeSummary() {
          return {
            title: this.heading(),
            h2s: this.h2s(),
            buttons: [...document.querySelectorAll('button')].filter((el) => this.visible(el)).map((el) => this.text(el)).filter(Boolean).slice(0, 80),
            body: this.text(document.body).slice(0, 1000),
            modal: Boolean(document.querySelector('.modal-backdrop')),
            formError: [...document.querySelectorAll('.form-error')].filter((el) => this.visible(el)).map((el) => this.text(el)).filter(Boolean),
          };
        },
      };
    })()`);
  }

  async click(text, selector) {
    const target = await this.eval(`window.__uiSmoke.targetBox(${JSON.stringify(text)}, ${selector ? JSON.stringify(selector) : 'undefined'})`);
    if (process.env.SLAN_UI_SMOKE_DEBUG) {
      console.log(`[ui-smoke] click ${this.label}: ${JSON.stringify(target)}`);
    }
    await this.send('Input.dispatchMouseEvent', { type: 'mouseMoved', x: target.x, y: target.y, button: 'none' });
    await this.send('Input.dispatchMouseEvent', { type: 'mousePressed', x: target.x, y: target.y, button: 'left', buttons: 1, clickCount: 1 });
    await this.send('Input.dispatchMouseEvent', { type: 'mouseReleased', x: target.x, y: target.y, button: 'left', clickCount: 1 });
    await sleep(500);
  }

  async clickRowButton(rowText, buttonText) {
    await this.eval(`window.__uiSmoke.clickRowButton(${JSON.stringify(rowText)}, ${JSON.stringify(buttonText)})`);
    await sleep(500);
  }

  async clickModalButton(text) {
    await this.eval(`window.__uiSmoke.clickModalButton(${JSON.stringify(text)})`);
    await sleep(500);
  }

  async waitRowText(rowText, expectedText, timeoutMs = 15000) {
    try {
      await waitFor(async () => this.eval(`(() => {
        const row = [...document.querySelectorAll('tr')].find((el) => window.__uiSmoke.visible(el) && window.__uiSmoke.text(el).includes(${JSON.stringify(rowText)}));
        return row ? window.__uiSmoke.text(row).includes(${JSON.stringify(expectedText)}) : false;
      })()`), timeoutMs, `${this.label} row ${rowText} text ${expectedText}`);
    } catch (error) {
      const summary = await this.eval('window.__uiSmoke.activeSummary()').catch(() => ({}));
      throw new Error(`${error.message}; summary=${JSON.stringify(summary)}; errors=${JSON.stringify({
        console: this.consoleErrors,
        page: this.pageErrors,
        network: this.networkErrors,
      })}`);
    }
  }

  async waitText(text, timeoutMs = 15000) {
    await waitFor(async () => this.eval(`document.body.innerText.includes(${JSON.stringify(text)})`), timeoutMs, `${this.label} text ${text}`);
  }

  async waitSelector(selector, timeoutMs = 15000) {
    try {
      await waitFor(async () => this.eval(`Boolean(document.querySelector(${JSON.stringify(selector)}))`), timeoutMs, `${this.label} selector ${selector}`);
    } catch (error) {
      const summary = await this.eval('window.__uiSmoke.activeSummary()').catch(() => ({}));
      throw new Error(`${error.message}; summary=${JSON.stringify(summary)}; errors=${JSON.stringify({
        console: this.consoleErrors,
        page: this.pageErrors,
        network: this.networkErrors,
      })}`);
    }
  }

  async waitNoSelector(selector, timeoutMs = 15000) {
    try {
      await waitFor(async () => this.eval(`!document.querySelector(${JSON.stringify(selector)})`), timeoutMs, `${this.label} no selector ${selector}`);
    } catch (error) {
      const summary = await this.eval('window.__uiSmoke.activeSummary()').catch(() => ({}));
      throw new Error(`${error.message}; summary=${JSON.stringify(summary)}; errors=${JSON.stringify({
        console: this.consoleErrors,
        page: this.pageErrors,
        network: this.networkErrors,
      })}`);
    }
  }

  async screenshot(name) {
    const shot = await this.send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: true });
    const file = path.join(outDir, `${this.label}-${name}.png`);
    await writeFile(file, Buffer.from(shot.data, 'base64'));
    return file;
  }

  clearNetworkErrorsContaining(pattern) {
    this.networkErrors = this.networkErrors.filter((item) => !item.includes(pattern));
  }

  clearErrorsContaining(pattern) {
    this.consoleErrors = this.consoleErrors.filter((item) => !item.includes(pattern));
    this.pageErrors = this.pageErrors.filter((item) => !item.includes(pattern));
    this.networkErrors = this.networkErrors.filter((item) => !item.includes(pattern));
  }

  async assertHealthy(context) {
    await sleep(250);
    const summary = await this.eval('window.__uiSmoke.activeSummary()');
    const visibleFormErrors = (summary.formError || []).filter((text) => text && !text.includes('请输入'));
    const errors = [
      ...this.consoleErrors,
      ...this.pageErrors,
      ...this.networkErrors,
      ...visibleFormErrors.map((text) => `form-error: ${text}`),
    ].filter(Boolean);
    if (errors.length > 0) {
      await this.screenshot(`failed-${context.replace(/[^a-zA-Z0-9_-]/g, '_')}`);
      throw new Error(`${this.label} ${context} failed: ${errors.join(' | ')}`);
    }
    if (!summary.body || summary.body.length < 30) {
      throw new Error(`${this.label} ${context} rendered empty body`);
    }
    return summary;
  }

  close() {
    this.ws?.close();
  }
}

async function newPage(port, url, label) {
  const target = await fetchJSON(`http://127.0.0.1:${port}/json/new?${encodeURIComponent('about:blank')}`, { method: 'PUT' });
  const page = new CDPPage(target.webSocketDebuggerUrl, label);
  await page.connect();
  await page.navigate(url);
  await page.installHelpers();
  return page;
}

async function launchChrome() {
  const port = 42000 + Math.floor(Math.random() * 10000);
  const userDataDir = await mkdtemp(path.join(os.tmpdir(), 'slan-ui-chrome-'));
  const args = [
    '--headless=new',
    `--remote-debugging-port=${port}`,
    `--user-data-dir=${userDataDir}`,
    '--no-first-run',
    '--no-default-browser-check',
    '--disable-background-networking',
    '--disable-gpu',
    'about:blank',
  ];
  const chrome = spawn(chromePath, args, { stdio: ['ignore', 'pipe', 'pipe'] });
  let stderr = '';
  chrome.stderr.on('data', (chunk) => { stderr += chunk.toString(); });
  await waitFor(async () => {
    try {
      const response = await fetch(`http://127.0.0.1:${port}/json/version`);
      return response.ok;
    } catch {
      return false;
    }
  }, 20000, `Chrome DevTools (${stderr.slice(0, 500)})`);
  return {
    port,
    async close() {
      if (!chrome.killed) {
        chrome.kill('SIGTERM');
      }
      await Promise.race([
        new Promise((resolve) => chrome.once('exit', resolve)),
        sleep(2000),
      ]);
      for (let attempt = 0; attempt < 5; attempt += 1) {
        try {
          await rm(userDataDir, { recursive: true, force: true, maxRetries: 3, retryDelay: 200 });
          return;
        } catch (error) {
          if (attempt === 4) throw error;
          await sleep(250);
        }
      }
    },
  };
}

async function exerciseCustomerUI(browser) {
  const page = await newPage(browser.port, webBase, 'web');
  const email = `ui-browser-${runID}@staticlss.com`;
  const testDeviceID = `ui-device-${runID}`;
  try {
    await page.waitText('SLAN Client Web');
    await page.click('注册');
    await page.eval(`window.__uiSmoke.setInput(0, 'UI Browser Smoke')`);
    await page.eval(`window.__uiSmoke.setInput(1, ${JSON.stringify(email)})`);
    await page.eval(`window.__uiSmoke.setInput(2, 'password123')`);
    await page.click('注册并进入主页');
    await page.waitSelector('.shell');
    await page.assertHealthy('customer after register');
    await page.eval(`(async () => {
      const auth = JSON.parse(localStorage.getItem('slan.clientWeb.auth') || 'null');
      if (!auth?.user?.userId) throw new Error('customer auth missing for device setup');
      const response = await fetch('/api/web/devices/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          userId: auth.user.userId,
          deviceId: ${JSON.stringify(testDeviceID)},
          name: 'UI Browser Test Device',
          platform: 'linux',
          osName: 'Linux',
          osVersion: '6.8',
          alias: 'UI Test Device',
          publicKey: 'ui-smoke-public-key-${runID}',
        }),
      });
      if (!response.ok && response.status !== 409) {
        throw new Error('register test device failed: HTTP ' + response.status + ' ' + await response.text());
      }
      location.href = '/overview';
    })()`);
    await page.waitSelector('.shell');
    await page.installHelpers();
    await page.assertHealthy('customer test device registered');

    const navs = ['控制台概览', '设备', '用户别名', '网络管理'];
    for (const nav of navs) {
      await page.click(nav, 'aside nav button');
      await page.waitText(nav);
      await page.assertHealthy(`customer nav ${nav}`);
      await page.screenshot(`nav-${nav}`);
    }

    const modalButtons = ['修改密码'];
    for (const button of modalButtons) {
      await page.click(button);
      await page.waitSelector('.modal-backdrop');
      await page.assertHealthy(`customer modal ${button}`);
      await page.eval('window.__uiSmoke.closeOverlays()');
    }

    await page.click('设备', 'aside nav button');
    await page.waitText(testDeviceID);
    console.log('[ui-smoke] web device modal: 生成安装命令');
    await page.click('生成安装命令');
    await page.waitSelector('.modal-backdrop');
    await page.waitText('一次性接入安装 Key');
    await page.assertHealthy('customer device bootstrap modal');
    await page.eval('window.__uiSmoke.closeOverlays()');

    console.log('[ui-smoke] web device modal: 生成接入码');
    await page.click('生成接入码');
    await page.waitSelector('.modal-backdrop');
    await page.waitText('接入字符串');
    const inviteCode = await page.eval(`document.querySelector('.modal-backdrop .invite-code strong')?.innerText?.trim() || ''`);
    if (!inviteCode) {
      throw new Error('customer device invite code missing');
    }
    await page.assertHealthy('customer device invite modal');
    await page.eval('window.__uiSmoke.closeOverlays()');

    const externalInviteCode = await page.eval(`(async () => {
      const suffix = ${JSON.stringify(String(runID))};
      const authResponse = await fetch('/api/web/auth/register', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: 'ui-inviter-' + suffix + '@staticlss.com',
          password: 'password123',
          name: 'UI Invite Owner',
        }),
      });
      if (!authResponse.ok) {
        throw new Error('create external inviter failed: HTTP ' + authResponse.status + ' ' + await authResponse.text());
      }
      const authPayload = await authResponse.json();
      const inviteResponse = await fetch('/api/web/device-invites', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          inviterUserId: authPayload.auth.user.userId,
          ttlSeconds: 86400,
        }),
      });
      if (!inviteResponse.ok) {
        throw new Error('create external invite failed: HTTP ' + inviteResponse.status + ' ' + await inviteResponse.text());
      }
      const invitePayload = await inviteResponse.json();
      return invitePayload.inviteCode || '';
    })()`);
    if (!externalInviteCode) {
      throw new Error('external invite code missing');
    }

    console.log('[ui-smoke] web device modal: 邀请确认');
    await page.click('邀请确认');
    await page.waitSelector('.modal-backdrop');
    await page.eval(`window.__uiSmoke.setValue(document.querySelector('.modal-backdrop input'), ${JSON.stringify(externalInviteCode)})`);
    await page.click('接入');
    await sleep(800);
    await page.assertHealthy('customer device invite confirm');

    const firstDeviceOpened = await page.eval(`(() => {
      const rowButton = [...document.querySelectorAll('td button.text-link')].find((el) => window.__uiSmoke.visible(el));
      if (!rowButton) return false;
      rowButton.click();
      return true;
    })()`);
    if (firstDeviceOpened) {
      await page.waitSelector('.modal-backdrop');
      await page.assertHealthy('customer device usage modal');
      await page.eval('window.__uiSmoke.closeOverlays()');
    }

    await page.click('用户别名', 'aside nav button');
    const aliasOpened = await page.eval(`(() => {
      const button = [...document.querySelectorAll('button')].find((el) => window.__uiSmoke.visible(el) && window.__uiSmoke.text(el).includes('修改'));
      if (!button) return false;
      button.click();
      return true;
    })()`);
    if (aliasOpened) {
      await sleep(300);
      await page.assertHealthy('customer user alias editor');
      await page.eval('window.__uiSmoke.closeOverlays()');
    }

    await page.click('网络管理', 'aside nav button');
    await page.click('新增网络');
    await page.waitSelector('.modal-backdrop');
    await page.assertHealthy('customer network create modal');
    await page.eval('window.__uiSmoke.closeOverlays()');
    const openedWorkspace = await page.eval(`(() => {
      const button = [...document.querySelectorAll('table button.text-link')].find((el) => window.__uiSmoke.visible(el));
      if (!button) return false;
      button.click();
      return true;
    })()`);
    if (openedWorkspace) {
      for (const tab of ['设备', '内网域名', '访问规则']) {
        await page.click(tab, '.panel-tabs button');
        await page.assertHealthy(`customer workspace tab ${tab}`);
        if (tab === '设备') {
          await page.waitText('网络设备');
          await page.waitText(testDeviceID);
          const exists = await page.eval(`Boolean(window.__uiSmoke.findClickable('添加设备'))`);
          if (exists) {
            await page.click('添加设备');
            await page.waitSelector('.modal-backdrop');
            await page.assertHealthy('customer workspace add device modal');
            await page.eval('window.__uiSmoke.closeOverlays()');
          }
        }
        if (tab === '内网域名') {
          const zoneName = `ui-${runID}.lan`;
          const recordName = `web-${runID}`;
          await page.click('新增域');
          await page.waitSelector('.modal-backdrop');
          await page.eval(`window.__uiSmoke.setValue(document.querySelector('.modal-backdrop input'), ${JSON.stringify(zoneName)})`);
          await page.assertHealthy('customer dns zone modal');
          await page.click('创建');
          await sleep(900);
          await page.waitText(zoneName);
          await page.click('解析');
          await page.waitText('解析管理');
          await page.click('新增解析');
          await page.waitSelector('.modal-backdrop');
          await page.eval(`(() => {
            const inputs = [...document.querySelectorAll('.modal-backdrop input')].filter((el) => window.__uiSmoke.visible(el));
            window.__uiSmoke.setValue(inputs[0], ${JSON.stringify(recordName)});
            window.__uiSmoke.setValue(inputs[1], '8443');
          })()`);
          await page.assertHealthy('customer dns record modal');
          await page.click('创建');
          await sleep(900);
          await page.waitText(recordName);
          await page.assertHealthy('customer dns record created');
        }
        if (tab === '访问规则') {
          const groupName = `UI ACL ${runID}`;
          await page.click('新增安全组');
          await page.waitSelector('.modal-backdrop');
          await page.eval(`window.__uiSmoke.setValue(document.querySelector('.modal-backdrop input'), ${JSON.stringify(groupName)})`);
          await page.assertHealthy('customer security group modal');
          await page.click('创建');
          await sleep(900);
          await page.waitText(groupName);
          await page.click(groupName);
          await page.waitText('新增入方向');
          await page.click('新增入方向');
          await page.waitSelector('.modal-backdrop');
          await page.assertHealthy('customer acl ingress rule modal');
          await page.click('创建');
          await sleep(900);
          await page.waitText('allow');
          await page.click('新增出方向');
          await page.waitSelector('.modal-backdrop');
          await page.assertHealthy('customer acl egress rule modal');
          await page.click('创建');
          await sleep(900);
          await page.assertHealthy('customer acl rules created');
        }
      }
    }

    await page.screenshot('final');
    return { email, screenshots: outDir };
  } finally {
    page.close();
  }
}

async function exerciseOpsUI(browser) {
  const page = await newPage(browser.port, opsBase, 'ops');
  try {
    await page.waitText('运营管理登录');
    await page.eval(`localStorage.clear()`);
    await page.navigate(opsBase);
    await page.installHelpers();
    await page.waitText('运营管理登录');
    await page.eval(`window.__uiSmoke.setInput(0, 'admin1')`);
    await page.eval(`window.__uiSmoke.setInput(1, 'admin1')`);
    await page.click('登录');
    await page.waitSelector('.shell');
    await page.assertHealthy('ops after login');
    await page.eval(`(async () => {
      const authResponse = await fetch(${JSON.stringify(new URL('/api/web/auth/register', webBase).toString())}, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          email: 'ops-ui-customer-${runID}@staticlss.com',
          password: 'password123',
          name: 'Ops UI Customer ${runID}',
        }),
      });
      if (!authResponse.ok && authResponse.status !== 409) {
        throw new Error('seed ops customer failed: HTTP ' + authResponse.status + ' ' + await authResponse.text());
      }
      const authPayload = authResponse.ok ? await authResponse.json() : null;
      const userId = authPayload?.auth?.user?.userId;
      if (userId) {
        const deviceResponse = await fetch(${JSON.stringify(new URL('/api/web/devices/register', webBase).toString())}, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            userId,
            deviceId: 'ops-ui-device-${runID}',
            name: 'Ops UI Device',
            platform: 'linux',
            osName: 'Linux',
            osVersion: '6.8',
            alias: 'Ops UI Device',
            publicKey: 'ops-ui-public-key-${runID}',
          }),
        });
        if (!deviceResponse.ok && deviceResponse.status !== 409) {
          throw new Error('seed ops device failed: HTTP ' + deviceResponse.status + ' ' + await deviceResponse.text());
        }
      }
    })()`);
    await page.click('刷新');
    await sleep(1000);
    await page.assertHealthy('ops seeded data loaded');

    const navs = ['运营管理', '运营用户', '中继节点', '客户管理', '设备管理', '客户端发布', '商品管理', '订单管理', '续费管理'];
    for (const nav of navs) {
      await page.click(nav, 'aside nav button');
      await page.waitText(nav);
      await page.assertHealthy(`ops nav ${nav}`);
      await page.screenshot(`nav-${nav}`);
    }

    await page.click('修改密码');
    await page.waitSelector('.modal-backdrop');
    await page.assertHealthy('ops current password modal');
    await page.eval('window.__uiSmoke.closeOverlays()');

    const modalMatrix = [
      ['运营用户', ['新增运营用户']],
      ['中继节点', ['新增中继节点']],
      ['商品管理', ['新增套餐', '新增商品']],
      ['订单管理', ['创建订单']],
      ['续费管理', ['手动续费']],
    ];
    for (const [nav, buttons] of modalMatrix) {
      await page.click(nav, 'aside nav button');
      for (const button of buttons) {
        const exists = await page.eval(`Boolean(window.__uiSmoke.findClickable(${JSON.stringify(button)}))`);
        if (!exists) continue;
        await page.click(button);
        await page.waitSelector('.modal-backdrop');
        await page.assertHealthy(`ops modal ${nav} ${button}`);
        await page.eval('window.__uiSmoke.closeOverlays()');
        await sleep(250);
      }
    }

    const operatorEmail = `ops-ui-${runID}@staticlss.com`;
    await page.click('运营用户', 'aside nav button');
    await page.click('新增运营用户');
    await page.waitSelector('.modal-backdrop');
    await page.eval(`(() => {
      const inputs = [...document.querySelectorAll('.modal-backdrop input')].filter((el) => window.__uiSmoke.visible(el));
      window.__uiSmoke.setValue(inputs[0], 'Ops UI ${runID}');
      window.__uiSmoke.setValue(inputs[1], ${JSON.stringify(operatorEmail)});
      window.__uiSmoke.setValue(inputs[2], 'ops-ui-pass-123');
      window.__uiSmoke.setValue(inputs[3], 'ops-ui-pass-123');
    })()`);
    await page.clickModalButton('保存');
    await page.waitNoSelector('.modal-backdrop');
    await page.waitText(operatorEmail);
    await page.assertHealthy('ops operator created');
    await page.clickRowButton(operatorEmail, '修改密码');
    await page.waitSelector('.modal-backdrop');
    await page.eval(`(() => {
      const inputs = [...document.querySelectorAll('.modal-backdrop input')].filter((el) => window.__uiSmoke.visible(el));
      window.__uiSmoke.setValue(inputs[0], 'ops-ui-pass-123');
      window.__uiSmoke.setValue(inputs[1], 'ops-ui-pass-123');
    })()`);
    await page.clickModalButton('确认修改');
    await page.waitNoSelector('.modal-backdrop');
    await page.assertHealthy('ops operator password updated');
    await page.clickRowButton(operatorEmail, '停用运营用户');
    await sleep(900);
    await page.waitRowText(operatorEmail, 'disabled');
    await page.assertHealthy('ops operator toggled');

    const relayName = `Ops UI Relay ${runID}`;
    await page.click('中继节点', 'aside nav button');
    await page.click('新增中继节点');
    await page.waitSelector('.modal-backdrop');
    await page.eval(`(() => {
      const inputs = [...document.querySelectorAll('.modal-backdrop input')].filter((el) => window.__uiSmoke.visible(el));
      window.__uiSmoke.setValue(inputs[0], ${JSON.stringify(relayName)});
      window.__uiSmoke.setValue(inputs[1], 'ui-test');
      window.__uiSmoke.setValue(inputs[2], '127.0.0.1');
      window.__uiSmoke.setValue(inputs[3], ${JSON.stringify(String(29110 + (runID % 1000)))});
      window.__uiSmoke.setValue(inputs[4], '800');
      window.__uiSmoke.setValue(inputs[5], '1024');
      window.__uiSmoke.setValue(inputs[6], '120');
    })()`);
    await page.clickModalButton('保存');
    await page.waitNoSelector('.modal-backdrop');
    await page.waitText(relayName);
    await page.assertHealthy('ops relay created');
    await page.clickRowButton(relayName, '停用中继节点');
    await sleep(900);
    await page.waitRowText(relayName, 'disabled');
    await page.assertHealthy('ops relay toggled');

    const planCode = `ops-ui-plan-${runID}`;
    const planName = `Ops UI Plan ${runID}`;
    const productName = `Ops UI Product ${runID}`;
    await page.click('商品管理', 'aside nav button');
    await page.click('新增套餐');
    await page.waitSelector('.modal-backdrop');
    await page.eval(`(() => {
      const inputs = [...document.querySelectorAll('.modal-backdrop input')].filter((el) => window.__uiSmoke.visible(el));
      window.__uiSmoke.setValue(inputs[0], ${JSON.stringify(planCode)});
      window.__uiSmoke.setValue(inputs[1], ${JSON.stringify(planName)});
      window.__uiSmoke.setValue(inputs[2], '8');
      window.__uiSmoke.setValue(inputs[3], '8');
      window.__uiSmoke.setValue(inputs[4], '16');
      window.__uiSmoke.setValue(inputs[5], '128');
      window.__uiSmoke.setValue(inputs[6], '80');
      window.__uiSmoke.setValue(inputs[7], '8');
      window.__uiSmoke.setValue(inputs[8], '19');
      window.__uiSmoke.setValue(inputs[9], '199');
    })()`);
    await page.clickModalButton('保存');
    await page.waitNoSelector('.modal-backdrop');
    await page.waitText(planName);
    await page.assertHealthy('ops plan created');
    await page.click('新增商品');
    await page.waitSelector('.modal-backdrop');
    await page.eval(`(() => {
      const inputs = [...document.querySelectorAll('.modal-backdrop input')].filter((el) => window.__uiSmoke.visible(el));
      const planSelect = [...document.querySelectorAll('.modal-backdrop select')].find((el) => [...el.options].some((option) => option.value === ${JSON.stringify(planCode)}));
      if (planSelect) window.__uiSmoke.setValue(planSelect, ${JSON.stringify(planCode)});
      window.__uiSmoke.setValue(inputs[0], ${JSON.stringify(productName)});
      window.__uiSmoke.setValue(inputs[1], '31');
      window.__uiSmoke.setValue(inputs[2], '128');
      window.__uiSmoke.setValue(inputs[3], '80');
      window.__uiSmoke.setValue(inputs[4], '29');
      window.__uiSmoke.setValue(inputs[5], '19');
      window.__uiSmoke.setValue(inputs[6], 'ops ui product');
    })()`);
    await page.clickModalButton('保存');
    await page.waitNoSelector('.modal-backdrop');
    await page.waitText(productName);
    await page.assertHealthy('ops product created');
    await page.clickRowButton(productName, '下架商品');
    await sleep(900);
    await page.waitRowText(productName, 'offline');
    await page.assertHealthy('ops product toggled');

    await page.click('订单管理', 'aside nav button');
    await page.click('创建订单');
    await page.waitSelector('.modal-backdrop');
    await page.eval(`(() => {
      const inputs = [...document.querySelectorAll('.modal-backdrop input')].filter((el) => window.__uiSmoke.visible(el));
      window.__uiSmoke.setValue(inputs[0], '19');
    })()`);
    await page.clickModalButton('保存');
    await page.waitNoSelector('.modal-backdrop');
    await page.waitText('manual');
    await page.assertHealthy('ops order created');

    await page.click('客户管理', 'aside nav button');
    await page.waitText(`ops-ui-customer-${runID}@staticlss.com`);
    await page.clickRowButton(`ops-ui-customer-${runID}@staticlss.com`, '指定客户级别');
    await page.waitSelector('.modal-backdrop');
    await page.eval(`(() => {
      const planSelect = [...document.querySelectorAll('.modal-backdrop select')].find((el) => [...el.options].some((option) => option.value === ${JSON.stringify(planCode)}));
      if (planSelect) window.__uiSmoke.setValue(planSelect, ${JSON.stringify(planCode)});
      const inputs = [...document.querySelectorAll('.modal-backdrop input')].filter((el) => window.__uiSmoke.visible(el));
      if (inputs[1]) window.__uiSmoke.setValue(inputs[1], '19');
    })()`);
    await page.clickModalButton('确认');
    await page.waitNoSelector('.modal-backdrop');
    await page.waitText(planName);
    await page.assertHealthy('ops customer plan assigned');

    await page.click('设备管理', 'aside nav button');
    await page.waitText(`ops-ui-device-${runID}`);
    await page.clickRowButton(`ops-ui-device-${runID}`, '查看或修改设备');
    await page.waitSelector('.modal-backdrop');
    await page.eval(`(() => {
      const inputs = [...document.querySelectorAll('.modal-backdrop input')].filter((el) => window.__uiSmoke.visible(el));
      window.__uiSmoke.setValue(inputs[0], 'Ops UI Device Updated');
    })()`);
    await page.clickModalButton('保存');
    await page.waitNoSelector('.modal-backdrop');
    await page.waitText('Ops UI Device Updated');
    await page.assertHealthy('ops device updated');
    await page.clickRowButton(`ops-ui-device-${runID}`, '停用设备');
    await sleep(900);
    await page.waitRowText(`ops-ui-device-${runID}`, '禁用');
    await page.assertHealthy('ops device toggled');

    await page.click('续费管理', 'aside nav button');
    await page.waitText(`ops-ui-customer-${runID}@staticlss.com`);
    await page.clickRowButton(`ops-ui-customer-${runID}@staticlss.com`, '查看或修改续费记录');
    await page.waitSelector('.modal-backdrop');
    await page.eval(`(() => {
      const inputs = [...document.querySelectorAll('.modal-backdrop input')].filter((el) => window.__uiSmoke.visible(el));
      const amount = inputs.find((el) => el.type === 'number');
      if (amount) window.__uiSmoke.setValue(amount, '29');
    })()`);
    await page.clickModalButton('保存');
    await page.waitNoSelector('.modal-backdrop');
    await page.waitText('29');
    await page.assertHealthy('ops renewal updated');

    await page.click('客户端发布', 'aside nav button');
    await page.eval(`(() => {
      const inputs = [...document.querySelectorAll('input')].filter((el) => window.__uiSmoke.visible(el));
      window.__uiSmoke.setValue(inputs[0], '9.9.${runID}');
      window.__uiSmoke.setValue(inputs[1], 'universal');
      const notes = inputs.find((el) => el.placeholder?.includes('修复内容'));
      if (notes) window.__uiSmoke.setValue(notes, 'ops ui smoke');
      const fileInput = document.querySelector('input[type="file"]');
      const file = new File(['ops-ui-smoke'], 'SLAN-Ops-UI-Smoke-${runID}.pkg', { type: 'application/octet-stream' });
      const transfer = new DataTransfer();
      transfer.items.add(file);
      fileInput.files = transfer.files;
      fileInput.dispatchEvent(new Event('change', { bubbles: true }));
    })()`);
    await page.click('上传发布');
    await sleep(1500);
    await page.waitText(`SLAN-Ops-UI-Smoke-${runID}.pkg`);
    await page.assertHealthy('ops client download uploaded');
    const uploadControls = await page.eval(`document.querySelectorAll('input,select').length`);
    if (uploadControls < 5) {
      throw new Error('ops client downloads upload controls missing');
    }
    await page.assertHealthy('ops client downloads controls');
    await page.screenshot('final');
    return { screenshots: outDir };
  } finally {
    page.close();
  }
}

let browser;
try {
  browser = await launchChrome();
  const only = process.env.SLAN_UI_SMOKE_ONLY || '';
  const customer = only === 'ops' ? null : await exerciseCustomerUI(browser);
  const ops = only === 'web' ? null : await exerciseOpsUI(browser);
  console.log(JSON.stringify({ status: 'ok', webBase, opsBase, customer, ops, outDir }, null, 2));
} finally {
  await browser?.close();
}
