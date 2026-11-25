'use strict';

// Design studio: image library, post templates, carousels and the Instagram post queue.
// Loaded before dashboard.js and uses its helpers (esc, icon, pageHead, panelHead, status,
// notify, rerenderFrom, render, pageName, main, session) only when called.

const newSlide = (over = {}) => ({template:'bottom',color:'#2a8f3a',headline:'',subline:'',stat:'',cta:'',image:null,focus:50,...over});
const designState = {
  loaded:false, loading:false, saving:false, state:'demo', instagram:false,
  folders:[], assets:[], posts:[], folder:'Brand', confirm:'',
  slides:[newSlide()], slide:0, logo:null, caption:''
};
const designTemplates = {bottom:'Photo, headline at bottom',center:'Photo, headline centered',solid:'Solid colour headline',stat:'Big number',image:'Image only'};
const designPhotoTemplates = ['bottom','center','image'];
const designColors = {'#2a8f3a':'Brand green','#0b6aa8':'Brand blue','#14305a':'Navy','#102a21':'Forest','#c9f76e':'Lime','#e1f4e6':'Mint'};
const designTypes = ['image/jpeg','image/png','image/webp'];
const designJson = {'Content-Type':'application/json'};
const designFont = 'system-ui, "Segoe UI", Arial, sans-serif';
const designCurrent = () => designState.slides[designState.slide];
const designCan = level => ({viewer:0,editor:1,admin:2})[session.role] >= level;

async function designApi(path,options = {}) {
  const response = await fetch(`/api/${path}`,{cache:'no-store',...options});
  if (!response.ok) throw new Error((await response.text()).trim() || `Request failed (${response.status})`);
  return response.status === 204 ? null : response.json();
}

async function loadDesign(force = false) {
  if (designState.loading) return;
  designState.loading = true;
  try {
    if (location.protocol === 'file:') throw new Error('offline');
    const payload = await designApi('design');
    const data = payload?.data;
    if (payload?.state === 'live' && Array.isArray(data?.folders) && Array.isArray(data.assets) && Array.isArray(data.posts)) {
      Object.assign(designState,{state:'live',folders:data.folders,assets:data.assets,posts:data.posts,instagram:payload.instagram === true});
      if (!data.folders.includes(designState.folder)) designState.folder = data.folders[0] ?? '';
      const known = id => typeof id !== 'string' || data.assets.some(asset => asset.id === id);
      for (const slide of designState.slides) if (!known(slide.image)) slide.image = null;
      if (!known(designState.logo)) designState.logo = null;
    } else designState.state = payload?.state === 'demo' ? 'demo' : 'error';
  } catch {
    designState.state = location.protocol === 'file:' ? 'demo' : 'error';
  }
  Object.assign(designState,{loaded:true,loading:false,confirm:''});
  const field = document.activeElement;
  const typing = field && main.contains(field) && ['INPUT','TEXTAREA','SELECT'].includes(field.tagName);
  if (pageName() === 'design' && (force || !typing)) render();
}

// An image source is either a library asset id (string) or a local Image from this browser.
const designLoaded = new Map(), designPending = new Map();
function designFetch(id) {
  const promise = new Promise(resolve => {
    const image = new Image();
    image.onload = () => { designLoaded.set(id,image); resolve(image); drawDesign(); };
    image.onerror = () => resolve(null);
    image.src = `/media/${encodeURIComponent(id)}`;
  });
  designPending.set(id,promise);
  return promise;
}
function designPeek(source) {
  if (!source) return null;
  if (typeof source !== 'string') return source.complete && source.naturalWidth ? source : null;
  if (!designLoaded.has(source) && !designPending.has(source)) designFetch(source);
  return designLoaded.get(source) ?? null;
}
const designEnsure = sources => Promise.all(sources.filter(Boolean).map(source => typeof source === 'string' ? designLoaded.get(source) ?? designPending.get(source) ?? designFetch(source) : source.decode().catch(() => null)));

const designRgb = hex => [1,3,5].map(index => parseInt(hex.slice(index,index + 2),16));
const designLuma = hex => { const [r,g,b] = designRgb(hex); return (.299 * r + .587 * g + .114 * b) / 255; };

function designWrap(ctx,text,width) {
  const lines = [];
  let line = '';
  for (const word of text.split(/\s+/).filter(Boolean)) {
    const attempt = line ? `${line} ${word}` : word;
    if (ctx.measureText(attempt).width > width && line) { lines.push(line); line = word; } else line = attempt;
  }
  if (line) lines.push(line);
  return lines;
}

// Largest type size that fits the text in the given number of lines.
function designFit(ctx,text,width,maxLines,max,min,weight) {
  for (let size = max; size >= min; size -= 4) {
    ctx.font = `${weight} ${size}px ${designFont}`;
    const lines = designWrap(ctx,text,width);
    if (lines.length <= maxLines) return {size,lines,weight};
  }
  ctx.font = `${weight} ${min}px ${designFont}`;
  return {size:min,lines:designWrap(ctx,text,width).slice(0,maxLines),weight};
}

// Bounding box of the non-white pixels, so logos exported with wide empty margins still fill their badge.
const designTrims = new WeakMap();
function designTrimBox(image) {
  if (designTrims.has(image)) return designTrims.get(image);
  const width = image.naturalWidth, height = image.naturalHeight;
  let box = {sx:0,sy:0,sw:width,sh:height};
  try {
    const canvas = document.createElement('canvas');
    canvas.width = width;
    canvas.height = height;
    const context = canvas.getContext('2d',{willReadFrequently:true});
    context.drawImage(image,0,0);
    const {data} = context.getImageData(0,0,width,height);
    let left = width, right = -1, top = height, bottom = -1;
    for (let y = 0; y < height; y++) for (let x = 0; x < width; x++) {
      const i = (y * width + x) * 4;
      if (data[i + 3] > 24 && (data[i] < 235 || data[i + 1] < 235 || data[i + 2] < 235)) {
        if (x < left) left = x;
        if (x > right) right = x;
        if (y < top) top = y;
        if (y > bottom) bottom = y;
      }
    }
    if (right > left && bottom > top) {
      const pad = Math.round(Math.max(right - left,bottom - top) * .02);
      const sx = Math.max(0,left - pad), sy = Math.max(0,top - pad);
      box = {sx,sy,sw:Math.min(width,right + pad) - sx,sh:Math.min(height,bottom + pad) - sy};
    }
  } catch { /* Keep the whole image if its pixels cannot be read. */ }
  designTrims.set(image,box);
  return box;
}

function drawSlide(canvas,slide,index,total) {
  const ctx = canvas.getContext('2d');
  const W = 1080, H = 1350, M = 80;
  const [r,g,b] = designRgb(slide.color);
  const ink = designLuma(slide.color) < .6 ? '#ffffff' : '#102a21';
  const template = slide.template;
  ctx.clearRect(0,0,W,H);
  ctx.fillStyle = slide.color;
  ctx.fillRect(0,0,W,H);
  const image = designPhotoTemplates.includes(template) ? designPeek(slide.image) : null;
  if (image) {
    const scale = Math.max(W / image.naturalWidth,H / image.naturalHeight);
    const w = image.naturalWidth * scale, h = image.naturalHeight * scale;
    ctx.drawImage(image,(W - w) * (slide.focus ?? 50) / 100,(H - h) / 2,w,h);
  }
  if (template === 'image') return;
  if (template === 'bottom' && image) {
    const fade = ctx.createLinearGradient(0,H * .35,0,H * .7);
    fade.addColorStop(0,`rgba(${r},${g},${b},0)`);
    fade.addColorStop(1,`rgba(${r},${g},${b},.97)`);
    ctx.fillStyle = fade;
    ctx.fillRect(0,H * .35,W,H * .65);
  } else if (template === 'center' && image) {
    ctx.fillStyle = `rgba(${r},${g},${b},.6)`;
    ctx.fillRect(0,0,W,H);
  }
  const overPhoto = template === 'bottom' && image;
  const markInk = overPhoto ? '#ffffff' : ink;
  ctx.textBaseline = 'alphabetic';
  ctx.textAlign = 'left';
  const logo = designPeek(designState.logo);
  if (logo) {
    const box = designTrimBox(logo);
    const scale = Math.min(72 / box.sh,360 / box.sw);
    const w = box.sw * scale, h = box.sh * scale;
    ctx.fillStyle = '#ffffff';
    ctx.beginPath();
    ctx.roundRect(M - 12,54,w + 24,h + 24,16);
    ctx.fill();
    ctx.drawImage(logo,box.sx,box.sy,box.sw,box.sh,M,66,w,h);
  } else {
    ctx.fillStyle = markInk;
    ctx.font = `800 36px ${designFont}`;
    ctx.fillText('AR ECOSYS',M,112);
  }
  if (total > 1) {
    ctx.fillStyle = markInk;
    ctx.font = `600 32px ${designFont}`;
    ctx.textAlign = 'right';
    if (overPhoto) { ctx.shadowColor = 'rgba(0,0,0,.6)'; ctx.shadowBlur = 10; }
    ctx.fillText(`${index + 1} / ${total}`,W - M,112);
    ctx.shadowColor = 'transparent';
    ctx.shadowBlur = 0;
  }
  const centered = template !== 'bottom';
  const width = W - 2 * M;
  const blocks = [];
  if (template === 'stat' && slide.stat.trim()) blocks.push({...designFit(ctx,slide.stat.trim(),width,1,360,120,800),leading:1,gap:0});
  if (slide.headline.trim()) blocks.push({...designFit(ctx,slide.headline.trim(),width,template === 'stat' ? 3 : 5,template === 'bottom' ? 112 : 120,template === 'stat' ? 44 : 56,800),leading:1.1,gap:blocks.length ? 28 : 0});
  if (slide.subline.trim()) blocks.push({...designFit(ctx,slide.subline.trim(),width,3,46,30,500),leading:1.35,gap:blocks.length ? 28 : 0,alpha:.92});
  const height = blocks.reduce((sum,block) => sum + block.gap + block.lines.length * block.size * block.leading,0);
  const cta = slide.cta.trim();
  const ctaBlock = cta ? 96 + 44 : 0;
  const bottom = H - 90 - ctaBlock;
  let y = template === 'bottom' ? bottom - height : Math.max(190,190 + (bottom - 190 - height) / 2);
  ctx.textAlign = centered ? 'center' : 'left';
  const x = centered ? W / 2 : M;
  ctx.fillStyle = ink;
  for (const block of blocks) {
    y += block.gap;
    ctx.font = `${block.weight} ${block.size}px ${designFont}`;
    ctx.globalAlpha = block.alpha ?? 1;
    for (const line of block.lines) { ctx.fillText(line,x,y + block.size * .8); y += block.size * block.leading; }
    ctx.globalAlpha = 1;
  }
  if (cta) {
    ctx.font = `700 40px ${designFont}`;
    const pillWidth = Math.min(width,ctx.measureText(cta).width + 96);
    const pillX = centered ? (W - pillWidth) / 2 : M, pillY = H - 90 - 96;
    ctx.fillStyle = slide.color === '#c9f76e' ? '#102a21' : '#c9f76e';
    ctx.beginPath();
    ctx.roundRect(pillX,pillY,pillWidth,96,48);
    ctx.fill();
    ctx.fillStyle = slide.color === '#c9f76e' ? '#c9f76e' : '#102a21';
    ctx.textAlign = 'center';
    ctx.fillText(cta,pillX + pillWidth / 2,pillY + 62,pillWidth - 40);
  }
}

function drawDesign() {
  const canvas = document.getElementById('d-canvas');
  if (canvas) drawSlide(canvas,designCurrent(),designState.slide,designState.slides.length);
}

function designBlob(slide,index) {
  const canvas = document.createElement('canvas');
  canvas.width = 1080;
  canvas.height = 1350;
  drawSlide(canvas,slide,index,designState.slides.length);
  return new Promise(resolve => canvas.toBlob(resolve,'image/jpeg',.92));
}

const designButton = (label,action,id = '',options = {}) => `<button type="button" class="button link ${options.danger ? 'danger' : ''}" data-action="${action}" data-id="${esc(id)}" ${options.disabled ? 'disabled' : ''}>${esc(label)}</button>`;
const designTime = value => value ? new Date(value).toLocaleString('en-US',{dateStyle:'medium',timeStyle:'short'}) : '—';
const designMin = () => { const date = new Date(Date.now() + 6e4); date.setMinutes(date.getMinutes() - date.getTimezoneOffset()); return date.toISOString().slice(0,16); };
const designAssetName = id => designState.assets.find(asset => asset.id === id)?.name ?? 'Library image';

function designPostActions(post) {
  const s = designState, off = !s.instagram || !designCan(2), edit = !designCan(1);
  const del = designButton(s.confirm === `post:${post.id}` ? 'Confirm delete' : 'Delete','d-delpost',post.id,{danger:true,disabled:edit});
  if (post.status === 'Draft' || post.status === 'Failed') return `${designButton('Approve','d-approve',post.id,{disabled:edit})}${del}`;
  if (post.status === 'Approved') return `<input type="datetime-local" data-when="${esc(post.id)}" min="${designMin()}" aria-label="Publish time" ${off ? 'disabled' : ''}>${designButton('Schedule','d-schedule',post.id,{disabled:off})}${designButton(s.confirm === `publish:${post.id}` ? 'Confirm publish' : 'Publish now','d-publish',post.id,{disabled:off})}${designButton('Unapprove','d-unapprove',post.id,{disabled:edit})}${del}`;
  if (post.status === 'Scheduled') return `${designButton(s.confirm === `publish:${post.id}` ? 'Confirm publish' : 'Publish now','d-publish',post.id,{disabled:off})}${designButton('Unschedule','d-unapprove',post.id,{disabled:edit})}${del}`;
  if (post.status === 'Publishing') return '<span class="muted">Publishing…</span>';
  return del;
}

function designQueue() {
  const s = designState;
  const rows = s.posts.map(post => {
    const slides = post.assets?.length > 1 ? `<br><span class="muted">Carousel, ${post.assets.length} slides</span>` : '';
    return `<tr><td><img class="post-thumb" src="/media/${encodeURIComponent(post.asset)}" alt="" loading="lazy"></td><td class="post-caption">${esc(post.caption.slice(0,90) || 'No caption')}${post.caption.length > 90 ? '…' : ''}${slides}${post.error ? `<br><span class="muted">${esc(post.error)}</span>` : ''}</td><td>${status(post.status === 'Publishing' ? 'Publishing…' : post.status,({Failed:'warn',Draft:'neutral',Publishing:'neutral'})[post.status] ?? '')}</td><td class="muted">${esc(designTime(post.status === 'Published' ? post.publishedAt : post.scheduledAt))}</td><td><div class="post-actions">${designPostActions(post)}</div></td></tr>`;
  }).join('');
  return `<section class="panel">${panelHead('Post queue','Draft, approve, then schedule or publish to Instagram')}${rows ? `<div class="table-wrap"><table class="data-table"><thead><tr><th scope="col">Image</th><th scope="col">Caption</th><th scope="col">Status</th><th scope="col">Time</th><th scope="col">Actions</th></tr></thead><tbody>${rows}</tbody></table></div>` : '<p class="empty">No posts yet. Design one above and save it as a draft.</p>'}</section>`;
}

function designLibrary() {
  const s = designState, live = s.state === 'live', edit = designCan(1);
  const assets = s.assets.filter(asset => asset.folder === s.folder);
  const grid = assets.length ? `<div class="asset-grid">${assets.map(asset => `<div class="asset"><button type="button" class="asset-pick" data-action="d-pick" data-id="${esc(asset.id)}" aria-pressed="${designCurrent().image === asset.id}" aria-label="Use ${esc(asset.name)} as the slide background"><img src="/media/${encodeURIComponent(asset.id)}" alt="${esc(asset.name)}" loading="lazy"></button><span class="asset-name">${esc(asset.name)}</span><div class="asset-actions">${designButton(s.logo === asset.id ? 'Logo ✓' : 'Use as logo','d-logo',asset.id,{disabled:!edit})}${designButton(s.confirm === `asset:${asset.id}` ? 'Confirm' : 'Delete','d-delasset',asset.id,{danger:true,disabled:!edit})}</div></div>`).join('')}</div>` : `<p class="empty">${live ? 'No images in this folder yet.' : 'Saved images appear here once design storage is configured.'}</p>`;
  const folders = live ? `<div class="design-row"><label class="control">${icon('file')}<select id="d-folder" aria-label="Folder">${s.folders.map(folder => `<option ${folder === s.folder ? 'selected' : ''}>${esc(folder)}</option>`).join('')}</select></label><label class="control"><input id="d-newfolder" maxlength="40" placeholder="New folder" aria-label="New folder name"></label><button type="button" class="button" data-action="d-addfolder" ${edit ? '' : 'disabled'}>Add folder</button></div>` : '';
  return `<section class="panel">${panelHead('Design library','Brand images, logos and finished posts')}<div class="design-body">${folders}<label class="control">Upload <input type="file" id="d-upload" accept="image/jpeg,image/png,image/webp" ${live ? 'multiple' : ''} ${edit ? '' : 'disabled'}></label>${grid}</div></section>`;
}

function designEditor() {
  const s = designState, slide = designCurrent(), live = s.state === 'live';
  const tags = s.caption.split(/\s+/).filter(word => word.startsWith('#')).length;
  const strip = `<div class="slide-strip" role="group" aria-label="Slides">${s.slides.map((_,index) => `<button type="button" class="slide-tab" data-action="d-slide" data-id="${index}" aria-pressed="${index === s.slide}" aria-label="Slide ${index + 1}">${index + 1}</button>`).join('')}${designButton('Add slide','d-addslide','',{disabled:s.slides.length >= 10})}${designButton('Duplicate','d-dupslide','',{disabled:s.slides.length >= 10})}${designButton('Move left','d-moveslide','-1',{disabled:s.slide === 0})}${designButton('Move right','d-moveslide','1',{disabled:s.slide === s.slides.length - 1})}${designButton('Remove','d-delslide','',{danger:true,disabled:s.slides.length === 1})}</div>`;
  const wantsImage = designPhotoTemplates.includes(slide.template);
  const imageNote = !wantsImage ? 'This layout uses a solid colour.' : slide.image ? `Background: ${typeof slide.image === 'string' ? esc(designAssetName(slide.image)) : 'image from this computer'}.` : 'Pick a background from the library or upload one.';
  const logoNote = s.logo ? `Logo: ${typeof s.logo === 'string' ? esc(designAssetName(s.logo)) : 'file from this computer'}` : 'No logo set, so the text mark is used';
  return `<section class="panel">${panelHead(s.slides.length > 1 ? `Create a carousel (${s.slides.length} slides)` : 'Create a post','1080 × 1350 images, saved as JPEG')}<div class="design-body">${strip}<div class="editor-grid"><div class="canvas-wrap"><canvas id="d-canvas" width="1080" height="1350" role="img" aria-label="Slide ${s.slide + 1} preview"></canvas></div><div class="design-fields">
    <label>Layout<select id="d-template">${Object.entries(designTemplates).map(([key,label]) => `<option value="${key}" ${slide.template === key ? 'selected' : ''}>${label}</option>`).join('')}</select></label>
    <label>Colour<select id="d-color">${Object.entries(designColors).map(([key,label]) => `<option value="${key}" ${slide.color === key ? 'selected' : ''}>${label}</option>`).join('')}</select></label>
    ${slide.template === 'stat' ? `<label>Number<input id="d-stat" maxlength="12" placeholder="e.g. 99.4%" value="${esc(slide.stat)}"></label>` : ''}
    <label>Headline<input id="d-headline" maxlength="140" value="${esc(slide.headline)}"></label>
    <label>Supporting line<input id="d-subline" maxlength="160" value="${esc(slide.subline)}"></label>
    <label>Button text (optional)<input id="d-cta" maxlength="40" placeholder="Book a ward pilot" value="${esc(slide.cta)}"></label>
    ${wantsImage && slide.image ? `<label>Photo position<input type="range" id="d-focus" min="0" max="100" value="${slide.focus ?? 50}" aria-label="Move the photo left or right"></label>` : ''}
    <p class="footnote">${imageNote} ${wantsImage && slide.image ? designButton('Clear image','d-clearimage') : ''}</p>
    <p class="footnote">${logoNote}. ${s.logo ? designButton('Clear logo','d-clearlogo') : ''} <label class="control inline-file">Logo file <input type="file" id="d-logofile" accept="image/jpeg,image/png,image/webp" ${designCan(1) ? '' : 'disabled'}></label></p>
  </div></div>
  <div class="design-fields"><label>Caption (shared by all slides)<textarea id="d-caption" maxlength="2200" rows="5">${esc(s.caption)}</textarea></label><small class="muted" id="d-count">${s.caption.length} / 2200 characters · ${tags} / 30 hashtags</small>
    <div class="design-row"><button type="button" class="button primary" data-action="d-save" ${live && !s.saving && designCan(1) ? '' : 'disabled'}>${s.saving ? 'Saving…' : s.slides.length > 1 ? 'Save carousel draft' : 'Save draft'}</button><button type="button" class="button" data-action="d-download">Download this slide</button></div></div></div></section>`;
}

function designPage() {
  const s = designState;
  if (!s.loaded && !s.loading) loadDesign();
  const note = !s.loaded ? '<p class="source-note">Loading the design library…</p>'
    : s.state === 'error' ? '<p class="source-note">The design library could not be loaded. Refresh to retry.</p>'
    : s.state === 'demo' ? '<p class="source-note">Design storage is not configured. Set ARECOSYS_MEDIA_DIR to keep images and drafts. You can still design and download post images.</p>'
    : s.instagram ? '' : '<p class="source-note">Instagram is not connected, so posts stay drafts. See the README to enable scheduling and publishing.</p>';
  return `${pageHead('Design studio','Build Instagram posts and carousels from your brand library, approve them and publish from here.')}${note}<div class="split">${designLibrary()}${designEditor()}</div>${s.state === 'live' ? designQueue() : ''}`;
}

function designUseLocal(file,target) {
  const image = new Image();
  image.onload = () => { drawDesign(); render(); };
  image.src = URL.createObjectURL(file);
  if (target === 'logo') designState.logo = image; else designCurrent().image = image;
}

async function designUpload(files,asLogo = false) {
  const s = designState;
  let uploaded = 0;
  for (const file of files) {
    if (!designTypes.includes(file.type) || file.size > 8 << 20) { notify(`${file.name} must be a JPEG, PNG or WebP image under 8 MB.`,true); continue; }
    if (s.state !== 'live') { designUseLocal(file,asLogo ? 'logo' : 'image'); return; }
    try {
      const name = file.name.replace(/\.[^.]+$/,'').slice(0,120);
      const asset = await designApi(`design/assets?folder=${encodeURIComponent(asLogo ? 'Brand' : s.folder)}&name=${encodeURIComponent(name)}`,{method:'POST',headers:{'Content-Type':file.type},body:file});
      if (asLogo) s.logo = asset.id;
      uploaded++;
    } catch (error) { notify(error.message,true); }
  }
  if (uploaded) { notify(uploaded === 1 ? 'Image uploaded.' : `${uploaded} images uploaded.`); await loadDesign(true); }
}

async function designSave() {
  const s = designState;
  if (s.saving) return;
  const missing = s.slides.findIndex(slide => designPhotoTemplates.includes(slide.template) && !slide.image);
  if (missing >= 0) { notify(`Slide ${missing + 1} needs a background image, or switch it to a solid colour or big number layout.`,true); return; }
  s.saving = true;
  render();
  try {
    await designEnsure([s.logo,...s.slides.map(slide => slide.image)]);
    const ids = [];
    for (const [index,slide] of s.slides.entries()) {
      const blob = await designBlob(slide,index);
      if (!blob) throw new Error('The image could not be exported.');
      const label = (slide.headline || slide.stat || 'Post').trim().slice(0,50) + (s.slides.length > 1 ? ` (${index + 1})` : '');
      ids.push((await designApi(`design/assets?folder=Posts&name=${encodeURIComponent(label)}`,{method:'POST',headers:{'Content-Type':'image/jpeg'},body:blob})).id);
    }
    await designApi('design/posts',{method:'POST',headers:designJson,body:JSON.stringify(ids.length === 1 ? {caption:s.caption,asset:ids[0]} : {caption:s.caption,assets:ids})});
    notify(ids.length > 1 ? 'Carousel draft saved to the post queue.' : 'Draft saved to the post queue.');
  } catch (error) { notify(error.message,true); }
  s.saving = false;
  await loadDesign(true);
}

async function designDownload() {
  const s = designState;
  await designEnsure([s.logo,designCurrent().image]);
  const blob = await designBlob(designCurrent(),s.slide);
  if (!blob) return;
  const link = document.createElement('a');
  link.href = URL.createObjectURL(blob);
  link.download = `arecosys-slide-${s.slide + 1}.jpg`;
  link.click();
  setTimeout(() => URL.revokeObjectURL(link.href),1000);
}

const designPatch = (id,body) => designApi(`design/posts/${encodeURIComponent(id)}`,{method:'PATCH',headers:designJson,body:JSON.stringify(body)});

async function designAction(action,id) {
  const s = designState, slide = designCurrent();
  const confirmed = key => s.confirm === key ? true : (s.confirm = key, rerenderFrom(`[data-action="${action}"][data-id="${id}"]`), false);
  try {
    if (action === 'd-pick') { slide.image = id; rerenderFrom(`[data-action="d-pick"][data-id="${id}"]`); return; }
    if (action === 'd-clearimage') { slide.image = null; render(); return; }
    if (action === 'd-logo') { s.logo = id; render(); return; }
    if (action === 'd-clearlogo') { s.logo = null; render(); return; }
    if (action === 'd-slide') { s.slide = Number(id); render(); return; }
    if (action === 'd-addslide') { s.slides.push(newSlide({template:slide.template === 'image' ? 'bottom' : slide.template,color:slide.color})); s.slide = s.slides.length - 1; render(); return; }
    if (action === 'd-dupslide') { s.slides.splice(s.slide + 1,0,{...slide}); s.slide++; render(); return; }
    if (action === 'd-delslide') { s.slides.splice(s.slide,1); s.slide = Math.min(s.slide,s.slides.length - 1); render(); return; }
    if (action === 'd-moveslide') {
      const to = s.slide + Number(id);
      [s.slides[s.slide],s.slides[to]] = [s.slides[to],s.slides[s.slide]];
      s.slide = to;
      render();
      return;
    }
    if (action === 'd-download') return await designDownload();
    if (action === 'd-save') return await designSave();
    if (action === 'd-addfolder') {
      const input = document.getElementById('d-newfolder');
      if (!input.value.trim()) { notify('Enter a folder name.',true); return; }
      await designApi('design/folders',{method:'POST',headers:designJson,body:JSON.stringify({name:input.value})});
      s.folder = input.value.trim();
    } else if (action === 'd-delasset') {
      if (!confirmed(`asset:${id}`)) return;
      await designApi(`design/assets/${encodeURIComponent(id)}`,{method:'DELETE'});
      notify('Image deleted.');
    } else if (action === 'd-delpost') {
      if (!confirmed(`post:${id}`)) return;
      await designApi(`design/posts/${encodeURIComponent(id)}`,{method:'DELETE'});
      notify('Post deleted.');
    } else if (action === 'd-approve') await designPatch(id,{action:'approve'});
    else if (action === 'd-unapprove') await designPatch(id,{action:'unapprove'});
    else if (action === 'd-schedule') {
      const value = document.querySelector(`[data-when="${id}"]`)?.value;
      if (!value) { notify('Choose a date and time first.',true); return; }
      await designPatch(id,{action:'schedule',scheduledAt:new Date(value).toISOString()});
      notify('Post scheduled.');
    } else if (action === 'd-publish') {
      if (!confirmed(`publish:${id}`)) return;
      await designPatch(id,{action:'publish'});
      notify('Publishing to Instagram…');
    } else return;
  } catch (error) { notify(error.message,true); }
  await loadDesign(true);
}

document.addEventListener('click',event => {
  const target = event.target.closest('[data-action^="d-"]');
  if (target && !target.disabled) designAction(target.dataset.action,target.dataset.id);
});
document.addEventListener('input',event => {
  const {id,value} = event.target;
  const fields = {'d-headline':'headline','d-subline':'subline','d-stat':'stat','d-cta':'cta'};
  if (fields[id]) { designCurrent()[fields[id]] = value; drawDesign(); }
  else if (id === 'd-focus') { designCurrent().focus = Number(value); drawDesign(); }
  else if (id === 'd-caption') {
    designState.caption = value;
    document.getElementById('d-count').textContent = `${value.length} / 2200 characters · ${value.split(/\s+/).filter(word => word.startsWith('#')).length} / 30 hashtags`;
  }
});
document.addEventListener('change',event => {
  const {id,value,files} = event.target;
  if (id === 'd-template') { designCurrent().template = value; rerenderFrom('#d-template'); }
  else if (id === 'd-color') { designCurrent().color = value; drawDesign(); }
  else if (id === 'd-folder') { designState.folder = value; rerenderFrom('#d-folder'); }
  else if (id === 'd-upload' && files.length) { designUpload([...files]); event.target.value = ''; }
  else if (id === 'd-logofile' && files.length) { designUpload([...files].slice(0,1),true); event.target.value = ''; }
});
if (location.protocol !== 'file:') setInterval(() => {
  if (location.hash === '#design' && !designState.loading && designState.posts.some(post => post.status === 'Publishing')) loadDesign();
},5000);
