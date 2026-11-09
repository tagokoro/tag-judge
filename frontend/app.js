const $=id=>document.getElementById(id); let version=1; let devMode=false; let cookieSession=false;
async function api(path, options={}){
  options={credentials:'same-origin',...options,headers:{...(options.headers||{})}};
  if(devMode){options.headers['X-User-ID']=$('dev-user-id').value;options.headers['X-User-Role']=$('dev-user-role').value;}
  if(options.body) options.headers['Content-Type']='application/json';
  const r=await fetch('/api/v1'+path,options), b=await r.json(); if(!r.ok) throw b; return b.data;
}
function show(id,v){$(id).textContent=JSON.stringify(v,null,2)}
async function refreshMe(){try{show('me-result',await api('/me'));cookieSession=true;devMode=false;$('use-dev-auth').checked=false}catch(e){cookieSession=false;show('me-result',e)}}
async function refreshLibrary(){try{show('library-detail',await api(`/namespaces/${$('namespace-slug').value}/libraries/${$('library-slug').value}`))}catch(e){show('library-detail',e)}}
$('google-login').onclick=()=>location.href='/api/v1/auth/google/login';
$('use-dev-auth').onchange=()=>{devMode=!cookieSession&&$('use-dev-auth').checked};
$('me').onclick=refreshMe;
$('create-namespace').onclick=async()=>{try{show('namespace-result',await api('/namespaces',{method:'POST',body:JSON.stringify({slug:$('namespace-slug').value,display_name:$('namespace-display-name').value})}))}catch(e){show('namespace-result',e)}};
$('create-library').onclick=async()=>{try{show('library-result',await api(`/namespaces/${$('namespace-slug').value}/libraries`,{method:'POST',body:JSON.stringify({slug:$('library-slug').value,name:$('library-name').value,description:$('library-description').value,visibility:$('library-visibility').value})}));await refreshLibrary()}catch(e){show('library-result',e)}};
$('library-get').onclick=refreshLibrary;
$('library-patch').onclick=async()=>{try{await api(`/namespaces/${$('namespace-slug').value}/libraries/${$('library-slug').value}`,{method:'PATCH',body:JSON.stringify({visibility:$('library-visibility').value})});await refreshLibrary()}catch(e){show('library-detail',e)}};
$('create-version').onclick=async()=>{try{const v=await api(`/namespaces/${$('namespace-slug').value}/libraries/${$('library-slug').value}/versions`,{method:'POST',body:JSON.stringify({source_code:$('version-source').value})});version=v.version_number;show('version-result',v)}catch(e){show('version-result',e)}};
$('start-verification').onclick=async()=>{try{let v=await api(`/namespaces/${$('namespace-slug').value}/libraries/${$('library-slug').value}/versions/${version}/verifications`,{method:'POST',body:JSON.stringify({problem_id:Number($('problem-id').value),adapter_source:$('adapter-source').value})});show('verification-result',v);for(let i=0;i<30&&!['ac','wa','tle','re','ce','mle','ie'].includes(v.status);i++){await new Promise(r=>setTimeout(r,1000));v=await api('/library-verifications/'+v.id);show('verification-result',v)}}catch(e){show('verification-result',e)}};
$('switch-user').onclick=async()=>{const id=$('dev-user-id');id.value=id.value==='00000000-0000-4000-8000-000000000001'?'00000000-0000-4000-8000-000000000002':'00000000-0000-4000-8000-000000000001';$('dev-user-role').value=id.value.endsWith('2')?'admin':'author';await refreshMe();await refreshLibrary()};
// Probe the cookie session first; dev headers are only enabled by an explicit checkbox selection.
refreshMe();
