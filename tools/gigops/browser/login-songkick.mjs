#!/usr/bin/env node
/** Visible human login bootstrap only; no site scraping, click automation,
 * event upload, publishing, or exports of cookies and passwords. */
import {createInterface} from 'node:readline/promises';
import {stdin,stdout} from 'node:process';
import {defaultProfilePath,preparePrivateProfile,PROJECT_ROOT,SONGKICK_TOURBOX_URL} from './profile.mjs';
async function main(){
 if(process.env.CI)throw new Error('Authenticated browser disabled in CI');
 if(process.platform==='linux'&&!process.env.DISPLAY&&!process.env.WAYLAND_DISPLAY){
   throw new Error('Graphical session required: use the laptop, not a headless cloud worker');
 }
 const dir=await preparePrivateProfile(process.env.MUSICOPS_BROWSER_PROFILE||defaultProfilePath(),PROJECT_ROOT);
 const {chromium}=await import('playwright');
 const browser=await chromium.launchPersistentContext(dir,{headless:false,viewport:{width:1280,height:850}});
 try{
   const page=browser.pages()[0]||await browser.newPage();
   await page.goto(SONGKICK_TOURBOX_URL,{waitUntil:'domcontentloaded',timeout:45000});
   stdout.write(`Tourbox opened in an isolated local profile: ${dir}\n`);
   stdout.write('Sign in and handle MFA yourself. Check whether Balkazar is already a managed artist.\n');
   stdout.write('This program does not inspect, upload or publish anything.\n');
   const io=createInterface({input:stdin,output:stdout});
   try{await io.question('Press Enter after manual checks to preserve session and close.\\n');}
   finally{io.close();}
 } finally {await browser.close();}
 stdout.write('Browser closed. The local session remains until Songkick expires it.\n');
}
main().catch(err=>{console.error('Songkick bootstrap: '+err.message);process.exitCode=1;});
