import os from 'node:os';
import path from 'node:path';
import fs from 'node:fs/promises';
import {fileURLToPath} from 'node:url';
export const SONGKICK_TOURBOX_URL='https://tourbox.songkick.com/';
export function defaultProfilePath(platform=process.platform,home=os.homedir()){
 if(platform==='win32')return path.join(process.env.LOCALAPPDATA||path.join(home,'AppData','Local'),'BalkazarMusicOps','songkick-chromium');
 if(platform==='darwin')return path.join(home,'Library','Application Support','BalkazarMusicOps','songkick-chromium');
 return path.join(home,'.local','share','balkazar-musicops','songkick-chromium');
}
function isWithin(parent,candidate){
 const relative=path.relative(parent,candidate);
 return relative===''||(!relative.startsWith(`..${path.sep}`)&&relative!=='..'&&!path.isAbsolute(relative));
}
export function validateProfilePath(profile,repoRoot){
 if(!profile||!path.isAbsolute(profile))throw new Error('Browser profile path must be absolute');
 if(isWithin(path.resolve(repoRoot),path.resolve(profile)))throw new Error('Refusing browser profile inside public repository');
 if(profile===path.parse(profile).root)throw new Error('Cannot use filesystem root as browser profile');
 return path.resolve(profile);
}
export async function preparePrivateProfile(profile,repoRoot){
 const p=validateProfilePath(profile,repoRoot);
 await fs.mkdir(p,{recursive:true,mode:0o700});
 if(process.platform!=='win32')await fs.chmod(p,0o700);
 return p;
}
export const PROJECT_ROOT=path.resolve(path.dirname(fileURLToPath(import.meta.url)),'..','..','..');
