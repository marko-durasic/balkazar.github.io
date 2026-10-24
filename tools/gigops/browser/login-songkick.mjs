#!/usr/bin/env node
/**
 * Supervised Songkick login opens a regular installed Chrome or Brave browser.
 * Google's OAuth can refuse browser automation, so we NEVER drive sign-in
 * through Playwright or enable remote debugging/stealth or scrape credentials.
 * No site automation, event upload, publishing, or browser-profile export.
 */
import path from 'node:path';
import {accessSync,constants} from 'node:fs';
import {spawn} from 'node:child_process';
import {defaultNativeProfilePath,preparePrivateProfile,PROJECT_ROOT,SONGKICK_TOURBOX_URL,nativeBrowserCandidates} from './profile.mjs';

function findExecutable(name) {
  const candidatePaths = path.isAbsolute(name)
    ? [name]
    : (process.env.PATH || '').split(path.delimiter).filter(Boolean).map(dir => path.join(dir,name));
  for (const file of candidatePaths) {
    try { accessSync(file,constants.X_OK); return file; }
    catch { /* not found: keep searching */ }
  }
  return null;
}
async function main() {
  if (process.env.CI) throw new Error('Interactive login is disabled in CI.');
  if (process.platform === 'linux' && !process.env.DISPLAY && !process.env.WAYLAND_DISPLAY)
    throw new Error('Open this from the graphical remote desktop, not plain SSH.');

  const candidates = process.env.MUSICOPS_NATIVE_BROWSER
    ? [process.env.MUSICOPS_NATIVE_BROWSER]
    : nativeBrowserCandidates();
  const browser = candidates.map(findExecutable).find(Boolean);
  if (!browser) throw new Error('No normal Google Chrome or Brave executable found. Use the Brave desktop shortcut instead.');

  const directory = await preparePrivateProfile(
    process.env.MUSICOPS_NATIVE_BROWSER_PROFILE || defaultNativeProfilePath(), PROJECT_ROOT
  );
  const args = [`--user-data-dir=${directory}`, '--new-window', SONGKICK_TOURBOX_URL];
  const child = spawn(browser,args,{stdio:'ignore',detached:true});
  await new Promise((resolve,reject)=>{
    child.once('spawn',resolve);
    child.once('error',reject);
  });
  child.unref();
  console.log(`Opened native browser: ${path.basename(browser)}`);
  console.log(`Separate private profile: ${directory}`);
  console.log('Sign in manually; complete Google/MFA normally and check whether Balkazar is a managed artist.');
  console.log('Do not create duplicate artists or events. Close the browser normally to retain the profile.');
  console.log('Nothing has been scraped, uploaded, published, or sent to an AI assistant.');
}
main().catch(error=>{
  console.error(`Songkick native browser launcher: ${error.message}`);
  process.exitCode=1;
});
