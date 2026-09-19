(() => {
  const t = window.yogilibT;
  const $ = id => document.getElementById(id), root = $('reader'), body = $('reader-body');
  if (!root) return;
  const status = text => {$('reader-status').textContent = text;};
  const endpoint = `/document/${root.dataset.document}`;
  async function api(path, method='GET', data) {
    const r = await fetch(endpoint+path,{method,headers:{'Content-Type':'application/json'},body:data ? JSON.stringify(data):undefined});
    if (!r.ok || r.redirected) throw Error(t("Could not complete this action. Check your connection and sign-in, then try again."));
    return r.status===204 || method!=='GET' ? null : r.json();
  }
  const settings = ['size','spacing','width'];
  let saved={}; try {saved=JSON.parse(localStorage.getItem('yogilib-reader-v2')||'{}');} catch {}
  settings.forEach(key=>{const input=$('reader-'+key); if(saved[key]) input.value=saved[key]; const update=()=>{root.style.setProperty('--reader-'+key,input.value+(key==='size'?'px':''));saved[key]=input.value;try{localStorage.setItem('yogilib-reader-v2',JSON.stringify(saved));}catch{}};input.addEventListener('input',update);update();});
  if(body) {
    const counts=new Map();
    body.querySelectorAll('h1,h2,h3,h4,p,li').forEach(el=>{
      const text=el.textContent.trim(); if(!text) return;
      let hash=2166136261; for(const ch of text) hash=Math.imul(hash^ch.codePointAt(0),16777619);
      const base='passage-'+(hash>>>0).toString(36), n=(counts.get(base)||0)+1; counts.set(base,n);
      if(!el.id) el.id=base+(n>1?'-'+n:'');
      if(/^H/.test(el.tagName)){const a=document.createElement('a');a.href='#'+el.id;a.textContent=text;$('reader-toc').append(a);}
      const a=document.createElement('a');a.href='#'+el.id;a.className='passage-link';a.textContent='¶';a.setAttribute('aria-label',t("Copy link to this passage"));
      a.addEventListener('click',async e=>{e.preventDefault();const url=new URL(location.href);url.hash=el.id;history.replaceState(null,'',url);try{await navigator.clipboard.writeText(url.href);status(t("Passage link copied."));}catch{status(t("Copy the passage link from your address bar."));}});el.append(a);
    });
    if(!$('reader-toc').children.length) $('reader-toc').textContent=t("No headings in this document yet.");
    if(location.hash) {try {document.getElementById(decodeURIComponent(location.hash.slice(1)))?.scrollIntoView();}catch{}}
  }
  function rangesFor(query) {
    if(!body || !query) return [];
    const walker=document.createTreeWalker(body,NodeFilter.SHOW_TEXT,{acceptNode:n=>n.parentElement.closest('.passage-link,script,style')?NodeFilter.FILTER_REJECT:NodeFilter.FILTER_ACCEPT});
    const nodes=[];let text='',n;while(n=walker.nextNode()){nodes.push({n,start:text.length});text+=n.textContent;}
    const result=[]; const pattern=new RegExp(query.replace(/[.*+?^${}()|[\]\\]/g,'\\$&'),'giu');let match;
    while((match=pattern.exec(text)) && result.length<1000){const pos=match.index,end=pos+match[0].length;const first=nodes.find(x=>x.start+x.n.length>pos),last=nodes.find(x=>x.start+x.n.length>=end);if(first&&last){const r=new Range();r.setStart(first.n,pos-first.start);r.setEnd(last.n,end-last.start);result.push(r);}}
    return result;
  }
  let matches=[],active=-1;
  function focusMatch(delta){if(!matches.length)return;active=(active+delta+matches.length)%matches.length;const r=matches[active];r.startContainer.parentElement.scrollIntoView({block:'center',behavior:'smooth'});if(window.Highlight&&CSS.highlights)CSS.highlights.set('reader-active',new Highlight(r));else{const s=getSelection();s.removeAllRanges();s.addRange(r);}$('search-count').textContent=`${active+1} / ${matches.length}`;}
  $('reader-search').addEventListener('submit',e=>{e.preventDefault();matches=rangesFor($('reader-query').value);active=-1;if(window.Highlight&&CSS.highlights){CSS.highlights.delete('reader-active');CSS.highlights.set('reader-search',new Highlight(...matches));}$('search-count').textContent=matches.length?'':t("No matches");focusMatch(1);});
  $('search-prev').onclick=()=>focusMatch(-1);$('search-next').onclick=()=>focusMatch(1);
  const original=$('reader-original');if(original){const file=original.dataset.file;if(/\.(png|jpe?g|gif|webp|avif)(\?|$)/i.test(file)){const img=document.createElement('img');img.src=file;img.alt=t("Original document scan");original.querySelector('object').replaceWith(img);} $('reader-split').onclick=()=>{const on=document.querySelector('.reader-columns').classList.toggle('split');$('reader-split').setAttribute('aria-pressed',on);};}
  let quote='';
  if($('capture-selection')) {
    document.addEventListener('selectionchange',()=>{const s=getSelection();if(s.rangeCount&&body?.contains(s.anchorNode)&&body.contains(s.focusNode)&&s.toString().trim()){const fragment=s.getRangeAt(0).cloneContents();fragment.querySelectorAll('.passage-link').forEach(el=>el.remove());quote=fragment.textContent.trim();$('selected-quote').textContent=quote;}});
    $('capture-selection').onclick=()=>status(quote?t("Passage selected. Add an optional note and save."):t("Select text in the document first."));
    async function loadNotes(){const notes=await api('/notes');$('notes-list').replaceChildren();const highlights=[];notes.forEach(n=>{highlights.push(...rangesFor(n.quote));const card=document.createElement('div');card.className='reader-note-card';const q=document.createElement('blockquote');q.textContent=n.quote;const p=document.createElement('p');p.textContent=n.note;const jump=document.createElement('button');jump.textContent=t("Find passage");jump.onclick=()=>{matches=rangesFor(n.quote);active=-1;if(matches.length)focusMatch(1);else status(t("This passage has changed since the note was saved."));};const remove=document.createElement('button');remove.textContent=t("Delete note");remove.onclick=async()=>{try{await api('/notes/'+n.id,'DELETE');await loadNotes();}catch(e){status(e.message);}};card.append(q,p,jump,remove);$('notes-list').append(card);});if(window.Highlight&&CSS.highlights)CSS.highlights.set('reader-notes',new Highlight(...highlights));if(!notes.length)$('notes-list').textContent=t("No saved highlights yet.");}
    $('save-note').onclick=async()=>{if(!quote){status(t("Select a passage first."));return;}try{await api('/notes','POST',{quote,note:$('reader-note').value});$('reader-note').value='';await loadNotes();status(t("Saved privately to your account."));}catch(e){status(e.message);}};
    loadNotes().catch(e=>status(e.message));
  }
  $('reader-history')?.addEventListener('toggle',async()=>{if(!$('reader-history').open)return;try{const revisions=await api('/revisions');$('history-list').replaceChildren();for(const v of revisions){const d=document.createElement('details'),summary=document.createElement('summary'),pre=document.createElement('p'),button=document.createElement('button');summary.textContent=`${v.date} — ${v.title}`;pre.textContent=v.text;pre.style.whiteSpace='pre-wrap';button.textContent=t("Restore this version");button.onclick=async()=>{if(!confirm(t("Restore this version? The current version will be saved in history.")))return;try{await api('/revisions/'+v.id,'POST');location.reload();}catch(e){status(e.message);}};d.append(summary,pre,button);$('history-list').append(d);}if(!revisions.length)$('history-list').textContent=t("No previous versions yet.");}catch(e){status(e.message);}});
})();
