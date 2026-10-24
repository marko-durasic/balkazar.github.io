import assert from 'node:assert/strict';
import {test} from 'node:test';
import path from 'node:path';
import os from 'node:os';
import {defaultProfilePath,validateProfilePath} from './profile.mjs';
test('profile default is absolute outside repository',()=>{
 for(const platform of ['linux','darwin','win32']){
  const p=defaultProfilePath(platform,path.join(os.tmpdir(),'owner'));
  assert.ok(path.isAbsolute(p));assert.ok(p.includes('songkick-chromium'));
 }
});
test('refuse browser cookies under public git repo',()=>{
 const repo=path.join(os.tmpdir(),'public-balkazar-repo');
 assert.throws(()=>validateProfilePath(path.join(repo,'tools','profile'),repo),/public repository/);
 assert.throws(()=>validateProfilePath(repo,repo),/public repository/);
 assert.throws(()=>validateProfilePath('relative',repo),/absolute/);
 assert.throws(()=>validateProfilePath(path.parse(repo).root,repo),/root/);
 assert.equal(validateProfilePath(path.join(os.tmpdir(),'private-profile'),repo),path.join(os.tmpdir(),'private-profile'));
});
