'use strict';

const icons = {
  brand:'M3 19 10 5h4l7 14M6 14h12M12 8v2m0 4v2', grid:'M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3zM14 14h7v7h-7z',
  chart:'M3 3v18h18M7 15l4-5 4 3 5-7', truck:'M2 5h13v12H2zM15 9h4l3 4v4h-7M5 17a2 2 0 1 0 4 0m8 0a2 2 0 1 0 4 0',
  people:'M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2M17 15a4 4 0 0 1 5 4v2M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0Zm3-4a4 4 0 0 1 0 8',
  instagram:'M7 3h10a4 4 0 0 1 4 4v10a4 4 0 0 1-4 4H7a4 4 0 0 1-4-4V7a4 4 0 0 1 4-4Zm9 9a4 4 0 1 1-8 0 4 4 0 0 1 8 0Zm1.5-5.5h.01',
  mail:'M3 5h18v14H3zM3 6l9 7 9-7', settings:'M4 6h16M4 12h16M4 18h16M8 4v4m8 2v4m-6 2v4', chevron:'m9 6 6 6-6 6', arrow:'M4 12h15m-5-5 5 5-5 5',
  calendar:'M4 5h16v16H4zM8 3v4m8-4v4M4 10h16', refresh:'M20 7v5h-5M4 17v-5h5M6 8a7 7 0 0 1 12-3l2 3M4 16l2 3a7 7 0 0 0 12-3',
  qr:'M3 3h6v6H3zM15 3h6v6h-6zM3 15h6v6H3zM15 15h3v3h3v3m0-6v.1M15 21v.1', gps:'M20 10c0 5-8 11-8 11S4 15 4 10a8 8 0 1 1 16 0Zm-5 0a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z',
  file:'M6 3h8l4 4v14H6zM14 3v5h4M9 12h6m-6 4h6', phone:'M7 2h10v20H7zM11 18h2', route:'M5 3a3 3 0 1 0 0 6 3 3 0 0 0 0-6Zm14 12a3 3 0 1 0 0 6 3 3 0 0 0 0-6ZM9 6h7a3 3 0 0 1 0 6H8a3 3 0 1 0 0 6h7',
  fuel:'M3 21V4h10v17M2 21h12M5 7h6v5H5zM13 13h2a2 2 0 0 1 2 2v3a2 2 0 0 0 4 0v-7l-4-4', bolt:'m13 2-8 12h6l-1 8 9-12h-6l1-8', close:'m5 5 14 14M19 5 5 19', send:'m21 3-7 18-3-8-8-3 18-7ZM11 13 21 3',
  search:'M15 15l5 5M17 10a7 7 0 1 1-14 0 7 7 0 0 1 14 0Z', alert:'M12 3 2 20h20L12 3Zm0 6v4m0 3v.1', check:'m4 12 5 5L20 6', globe:'M22 12a10 10 0 1 1-20 0 10 10 0 0 1 20 0ZM2 12h20m-10-10a15 15 0 0 1 0 20m0-20a15 15 0 0 0 0 20'
};
const icon = name => `<svg class="icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="${icons[name] || icons.chart}"/></svg>`;
const esc = value => String(value ?? '').replace(/[&<>"']/g, character => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[character]);
const fmt = value => typeof value === 'number' ? value.toLocaleString('en-US') : value ?? '—';
const tile = (name, color = '') => `<span class="tile-icon ${color}">${icon(name)}</span>`;
const session = {name:'',role:'admin'};
const can = level => ({viewer:0,editor:1,admin:2})[session.role] >= level;
const csvCell = value => {
  let text = String(value ?? '');
  if (/^[=+\-@\t\r]/.test(text)) text = `'${text}`;
  return /[",\n\r]/.test(text) ? `"${text.replaceAll('"','""')}"` : text;
};
function downloadCsv(name,rows) {
  const blob = new Blob(['﻿' + rows.map(row => row.map(csvCell).join(',')).join('\r\n')],{type:'text/csv;charset=utf-8'});
  const link = document.createElement('a');
  link.href = URL.createObjectURL(blob);
  link.download = name;
  link.click();
  setTimeout(() => URL.revokeObjectURL(link.href),1000);
}

// Live responses use the same fields as these fixtures. See README.md for the contracts.
const demoFeatures = [
  {id:'qr',name:'QR-verified pickups',metric:'Verified pickups',value:1238,status:'On track',details:{'Verification rate':'97.2%','Flagged scans':12,'Missing photo proof':7}},
  {id:'gps',name:'Live GPS telemetry',metric:'Vehicles reporting',value:'46 / 48',status:'Needs attention',details:{'Offline vehicles':2,'Last-position threshold':'5 min','Vehicles in motion':31}},
  {id:'file',name:'Digital waybills',metric:'Waybills issued',value:872,status:'On track',details:{'Signed waybills':842,'Awaiting signature':30}},
  {id:'phone',name:'Driver mobile app',metric:'Active drivers',value:64,status:'On track',details:{'App adoption':'89%','Pending syncs':3}},
  {id:'route',name:'ERP route scheduling',metric:'Routes scheduled',value:286,status:'On track',details:{'On-time completion':'94.6%','Route exceptions':8,'Unassigned routes':3}},
  {id:'fuel',name:'Diesel logs & alerts',metric:'Fleet fuel economy',value:'4.8 km/L',status:'Needs attention',details:{'Open km/L alerts':3,'Alert threshold':'4.2 km/L','Fuel logs':194}},
  {id:'bolt',name:'EV fleet',metric:'EVs ready for dispatch',value:'10 / 12',status:'On track',details:{'Charging':2,'Average charge':'81%','Charge-window conflicts':1,'EV inquiries':7}},
  {id:'generators',name:'Bulk waste generators',metric:'Active QR-tagged sites',value:128,status:'On track',details:{'Sites below target':9,'Sites onboarded this period':14}},
  {id:'plant',name:'Plant intake & custody',metric:'Loads reconciled',value:'831 / 842',status:'Needs attention',details:{'Open weighbridge mismatches':11,'Gate check-in (median)':'2 min 10 sec','Net tonnes received':924}},
  {id:'waste',name:'Waste streams & recovery',metric:'Tonnes diverted',value:428,status:'On track',details:{'Wet':'382 t','Dry':'196 t','Plastic':'64 t','Reject':'47 t','Pending ESG exports':2}},
  {id:'reliability',name:'Ward service reliability',metric:'Missed stops',value:16,status:'Needs attention',details:{'Late stops':32,'Open service complaints':6,'Vehicles due for maintenance':3}}
];
const demoDrilldowns = {
  reliability:[{Ward:'Ward 4 · Commercial core',Missed:6,Late:11,'Main cause':'Blocked access'},{Ward:'Ward 7 · Riverside corridor',Missed:4,Late:9,'Main cause':'Vehicle breakdown'},{Ward:'Ward 11 · Industrial belt',Missed:6,Late:12,'Main cause':'Late dispatch'}],
  gps:[{Vehicle:'WF-117',Shift:'B','Last ping':'38 min ago','Last ward':'Ward 7'},{Vehicle:'WF-124',Shift:'B','Last ping':'2 h ago','Last ward':'Ward 11'}],
  fuel:[{Vehicle:'WF-203','km/L':3.6,Threshold:4.2,'Idle time':'1 h 40 min'},{Vehicle:'WF-211','km/L':3.9,Threshold:4.2,'Idle time':'1 h 05 min'},{Vehicle:'WF-232','km/L':4.1,Threshold:4.2,'Idle time':'48 min'}],
  bolt:[{Vehicle:'EV-04','Charge ready':'06:40','Dispatch time':'06:00',Gap:'40 min late'}],
  plant:Array.from({length:11},(_,index) => {
    const over = index % 3 !== 0, field = 2.4 + index * .13, gap = over ? .21 : -.17;
    return {Load:`LD-${4410 + index}`,'Field weight':`${field.toFixed(2)} t`,'Plant weight':`${(field + gap).toFixed(2)} t`,Variance:`${over ? '+' : '-'}${Math.abs(gap).toFixed(2)} t`};
  })
};
const ago = hours => new Date(Date.now() - hours * 36e5).toISOString();
const demoLeads = [
  {id:'a1',name:'Priya Shah',company:'Westbay Distribution',email:'priya@example.test',source:'Website',interest:'Fleet management demo',status:'New',assignee:'',created:ago(30),note:'Interested in GPS tracking and driver app access for 35 vehicles.'},
  {id:'a2',name:'Arjun Mehta',company:'Northline Logistics',email:'arjun@example.test',source:'Instagram',interest:'EV fleet pricing',status:'New',assignee:'Meera',created:ago(3),note:'Asked about availability and charging requirements.'},
  {id:'a3',name:'Nisha Rao',company:'Harbor Freight Services',email:'nisha@example.test',source:'Email',interest:'Digital waybills',status:'Contacted',assignee:'Meera',created:ago(50),responded:ago(44),note:'Needs a quote for electronic proof of delivery.'},
  {id:'a4',name:'Dev Patel',company:'Greenlane Supply',email:'dev@example.test',source:'Website',interest:'Route scheduling',status:'Qualified',assignee:'Ravi',created:ago(80),responded:ago(70),note:'Planning a multi-depot rollout this quarter.'}
];
for (const lead of demoLeads) lead.activity = [{at:lead.created,by:'system',text:`Received via ${lead.source}`},...(lead.responded ? [{at:lead.responded,by:lead.assignee || 'team',text:`Stage: New to ${lead.status}`}] : [])];
demoLeads[1].phone = '+91 98765 43210';
const demoInbox = [
  {id:'mail-1',name:'Nisha Rao',email:'nisha@example.test',subject:'Digital waybills for our delivery network',body:'Hello, could you share how digital waybills work with proof of delivery and whether they can be reviewed by our dispatch team?',date:'Today',unread:true},
  {id:'mail-2',name:'Arjun Mehta',email:'arjun@example.test',subject:'EV fleet availability',body:'We are planning an EV pilot for regional deliveries. Could you share vehicle availability and charging requirements?',date:'Yesterday',unread:true},
  {id:'mail-3',name:'Dispatch team',email:'dispatch@example.test',subject:'GPS coverage on the evening routes',body:'Can we review the vehicles that stopped reporting during the evening shift?',date:'Yesterday',unread:false}
];
const ranges = {7:'Last 7 days',30:'Last 30 days',90:'Last 90 days'};
const demoSeries = {
  7:{traffic:[260,310,280,370,335,440,480],leads:[1,3,2,5,4,7,6],reach:[1320,1620,1480,1900,1800,2300,2320]},
  30:{traffic:[260,330,290,410,385,470,415,540,517,650,590,735,698,812],leads:[2,3,1,5,4,6,3,5,4,6,5,8,6,9],reach:[1260,1550,1490,1670,1820,1680,2130,1970,2330,2420,2210,2730,2550,2950]},
  90:{traffic:[810,990,890,1120,1050,1390,1290,1460,1370,1720,1600,1910,1820,2140],leads:[5,6,7,6,8,9,7,10,8,12,9,13,14,16],reach:[3100,3160,3540,3270,4190,4410,3870,4700,4500,4930,4750,5340,5100,5690]}
};

const channelMeta = {organic:['search','', 'Organic search'],direct:['globe','blue','Direct traffic'],social:['instagram','pink','Social referrals'],referral:['route','lavender','Referrals']};
const sample = days => {
  const {traffic,leads,reach} = demoSeries[days];
  const sum = values => values.reduce((total,value) => total + value, 0);
  const sessions = sum(traffic);
  const points = values => values.map((value,index) => {
    const date = new Date();
    date.setDate(date.getDate() - Math.round((days - 1) * (1 - index / (values.length - 1))));
    return {label:date.toLocaleDateString('en-US',{month:'short',day:'numeric'}),value};
  });
  const sitePages = [['Home','/',.58],['How It Works & FAQ','/how-it-works',.36],['ERP overview','/erp',.32],['Why Tirupati?','/why-tirupati',.23],['EV Fleet','/ev-fleet',.22],['About Us','/about-us',.17],['System Modules','/modules',.13],['Operational Impact','/impact',.11],['Contact Us','/contact-us',.08]];
  const months = Array.from({length:6},(_,index) => { const date = new Date(); date.setMonth(date.getMonth() - 5 + index); return date.toLocaleDateString('en-US',{month:'short'}); });
  const esg = {periods:months.map((label,index) => ({label,Wet:300 + index * 18 + index % 2 * 9,Dry:150 + index * 9,Plastic:44 + index * 3,Reject:52 - index * 2})),pendingExports:2};
  const actions = [['Homepage signups',.43],['Contact form submissions',.37],['WhatsApp clicks',.58],['Phone & email clicks',.24],['ERP & ward pilot CTA clicks',.71],['EV inquiry CTA clicks',.31]];
  return {website:{sessions,leads:sum(leads),pageviews:Math.round(sessions * 2.25),previous:{sessions:Math.round(sessions * .88),pageviews:Math.round(sessions * 2.25 * .9),leads:Math.round(sum(leads) * .8)},traffic:points(traffic),inquiries:points(leads),channels:{organic:42,direct:28,social:19,referral:11},pages:sitePages.map(([name,path,share])=>({name,path,views:Math.round(sessions*share)})),actions:actions.map(([name,share])=>({name,count:Math.round(sum(leads)*share)}))},instagram:{reach:{7:8400,30:24800,90:65200}[days],engagements:Math.round(sum(reach)*.042),followers:3264,profileVisits:Math.round(sum(reach)*.062),previous:{reach:Math.round({7:8400,30:24800,90:65200}[days] * .91),engagements:Math.round(sum(reach) * .042 * .88),profileVisits:Math.round(sum(reach) * .062 * .93)},series:points(reach)},fleet:{features:demoFeatures,drilldowns:demoDrilldowns,esg,wards:[{name:'Ward 4 · Commercial core',completed:154,scheduled:160,missed:6},{name:'Ward 7 · Riverside corridor',completed:126,scheduled:130,missed:4},{name:'Ward 11 · Industrial belt',completed:142,scheduled:148,missed:6}]},leads:demoLeads,inbox:demoInbox};
};

const pages = {overview:'Overview',website:'Website analytics',fleet:'Fleet operations',leads:'Leads',instagram:'Instagram reach',design:'Design studio',inbox:'Email inbox',esg:'ESG & recovery',activity:'Activity log',sources:'Data connections'};
const sourceNames = {website:'Website analytics',fleet:'Fleet systems',instagram:'Instagram Insights',leads:'Lead intake',inbox:'Email mailbox'};
const main = document.querySelector('main');
const dialog = document.querySelector('dialog');
const sources = {};
let days = 30, chartMetric = 'traffic', featureFilter = 'All', leadFilter = 'All', selectedMessage = '', loading = false, requestNumber = 0, noticeTimer;
const readMessages = new Set();
let leadSearch = '', leadSource = 'All', inboxSearch = '', auditRows = null, auditLoading = false;
const refreshedAt = {};
const sentReplies = {};
const pendingReplies = new Set();
const demoStages = {};
const demoOwners = {};
const replyDrafts = {};
const dashboard = () => ({...sample(days), ...Object.fromEntries(Object.entries(sources).filter(([,source]) => source.state !== 'demo').map(([key,source]) => [key, source.data]))});
const leadView = lead => sources.leads?.state === 'live' ? lead : {...lead,status:demoStages[lead.id] || lead.status,assignee:demoOwners[lead.id] ?? lead.assignee};
const dueAt = created => {
  const due = new Date(created);
  if (Number.isNaN(due.getTime())) return null;
  do due.setDate(due.getDate() + 1); while ([0,6].includes(due.getDay()));
  return due;
};
const isOverdue = lead => { const due = lead.status === 'New' && !lead.responded && dueAt(lead.created); return !!due && due < new Date(); };
const span = ms => { const hours = Math.max(1,Math.round(Math.abs(ms) / 36e5)); return hours >= 48 ? `${Math.round(hours / 24)} days` : `${hours} h`; };
const responseCell = lead => {
  if (lead.status !== 'New' || lead.responded) return status('Responded','neutral');
  const due = dueAt(lead.created);
  if (!due) return '<span class="muted">—</span>';
  const left = due - new Date();
  return status(left < 0 ? `Overdue ${span(left)}` : `Due in ${span(left)}`,left < 0 ? 'warn' : '');
};
const marketingPages = ['website','instagram'];
function setMarketingOpen(open,remember = false) {
  const toggle = document.querySelector('#marketing-group .nav-toggle');
  toggle.setAttribute('aria-expanded',String(open));
  document.querySelector('#nav-marketing').hidden = !open;
  if (remember) { try { localStorage.setItem('marketing-open',String(open)); } catch { /* Not remembered. */ } }
}
const pageName = () => Object.hasOwn(pages,location.hash.slice(1)) ? location.hash.slice(1) : 'overview';
const status = (label, type = '') => `<span class="pill ${type}"><span></span>${esc(label)}</span>`;
const button = (label, glyph, action, className = '') => `<button type="button" class="button ${className}" data-action="${action}">${glyph ? icon(glyph) : ''}${label}</button>`;
const pageHead = (title, subtitle, date = false, extra = '') => `<div class="page-head"><div><h1>${title}</h1><p>${subtitle}</p></div><div class="controls">${date ? `<label class="control">${icon('calendar')}<select id="range" aria-label="Date range">${Object.entries(ranges).map(([key,label]) => `<option value="${key}" ${Number(key) === days ? 'selected' : ''}>${label}</option>`).join('')}</select></label>` : ''}${extra}${button(loading ? 'Refreshing…' : 'Refresh','refresh','refresh')}</div></div>`;
const panelHead = (title, subtitle, action = '') => `<div class="panel-head"><div><h2>${title}</h2>${subtitle ? `<p>${subtitle}</p>` : ''}</div>${action}</div>`;
const noData = name => `<p class="empty">${sources[name]?.state === 'error' ? 'The source is unavailable. Refresh to retry.' : 'No records to display.'}</p>`;
const sitePageLink = row => /^\/[a-z0-9/-]*$/i.test(row.path) && !row.path.startsWith('//') ? `<a class="site-link" href="https://www.arecosys.com${esc(row.path)}" target="_blank" rel="noopener noreferrer">${esc(row.name)}</a>` : esc(row.name);
const updatedAt = name => refreshedAt[name] ? refreshedAt[name].toLocaleTimeString('en-US',{hour:'numeric',minute:'2-digit'}) : '';
const provenance = name => sources[name]?.state === 'live' ? (updatedAt(name) ? `<p class="source-note">Connected · updated ${updatedAt(name)}</p>` : '') : `<p class="source-note">${sources[name]?.state === 'error' ? 'The configured source could not be loaded.' : 'Sample data · Connect a data source to see your records.'}</p>`;
const card = (label,value,hint,glyph,color,open,series,previous) => {
  const delta = Number.isFinite(value) && Number.isFinite(previous) && previous > 0 ? (value - previous) / previous * 100 : null;
  const change = delta === null ? '' : `<span class="delta ${delta >= 0 ? 'up' : 'down'}">${delta >= 0 ? '▲' : '▼'} ${Math.abs(delta).toFixed(1)}% vs previous period</span>`;
  const numbers = (series || []).map(point => point.value).filter(Number.isFinite).slice(-14);
  const low = Math.min(...numbers), high = Math.max(...numbers);
  const points = numbers.map((number,index) => `${1+index*55/(numbers.length-1)},${21-(number-low)*18/(high-low || 1)}`).join(' ');
  const spark = numbers.length > 1 ? `<svg class="spark" viewBox="0 0 57 24" aria-hidden="true"><polyline points="${points}" fill="none" stroke="${color === 'pink' ? '#ac5b82' : '#48a276'}" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>` : '';
  return `<a class="card" href="#${open}"><span class="card-title">${tile(glyph,color)}${label}</span><strong>${esc(fmt(value))}</strong><small>${hint}</small>${change}${spark}</a>`;
};
const sourceHint = name => sources[name]?.state === 'live' ? 'Connected source' : sources[name]?.state === 'error' ? 'Source unavailable' : 'Sample data';

const hero = `<section class="hero"><div class="hero-copy"><span class="kicker">ARECOSYS OPERATIONS</span><h2>From first inquiry to final mile.</h2><p>Track how people find you and what keeps your fleet moving.</p><a href="#leads" class="button primary">Review new leads ${icon('arrow')}</a></div><div class="hero-art" aria-hidden="true"><svg viewBox="0 0 520 150" preserveAspectRatio="xMidYMid slice"><path d="M35 130 155 49 232 91 336 27 473 100m-442 7 132-86 68 38 106-64m-93 163 113-76 96 53 80-50" fill="none" stroke="currentColor" stroke-opacity=".2"/><path class="route-motion" d="M42 143 155 63 233 108 352 36 478 97" fill="none" stroke="currentColor" stroke-width="2.5"/><g fill="currentColor"><circle cx="155" cy="63" r="4"/><circle cx="233" cy="108" r="4"/><circle cx="352" cy="36" r="5"/><circle cx="478" cy="97" r="5"/></g><g fill="none" stroke="currentColor" stroke-opacity=".5"><circle cx="352" cy="36" r="12"/><circle cx="478" cy="97" r="12"/></g></svg></div></section>`;

function chart(title,subtitle,points,label,withToggle = false) {
  const toggle = withToggle ? `<div class="segmented" aria-label="Select chart metric">${[['traffic','Sessions'],['leads','Inquiries']].map(([key,name]) => `<button data-action="chart" data-value="${key}" aria-pressed="${chartMetric === key}">${name}</button>`).join('')}</div>` : '';
  const header = panelHead(title,subtitle,toggle);
  points = points?.filter(point => typeof point.label === 'string' && Number.isFinite(point.value) && point.value >= 0);
  if (!points?.length) return `<section class="panel">${header}<p class="empty">No chart data for this period.</p></section>`;
  const width = 590, top = 11, bottom = 128, left = 42, right = 575;
  const peak = Math.max(1,...points.map(point => point.value));
  const magnitude = 10 ** Math.floor(Math.log10(peak));
  const max = Math.ceil(peak / magnitude * 2) * magnitude / 2;
  const coords = points.map((point,index) => [left + (right-left)*index/Math.max(1,points.length-1),bottom-(bottom-top)*point.value/max]);
  const path = coords.map(([x,y],index) => `${index ? 'L' : 'M'}${x.toFixed(1)} ${y.toFixed(1)}`).join(' ');
  const step = Math.ceil(points.length/5);
  const marks = points.map((point,index) => {
    const [x,y] = coords[index];
    return `<circle class="hit" cx="${x}" cy="${y}" r="7" fill="transparent" tabindex="0" role="img" aria-label="${esc(point.label)}, ${esc(fmt(point.value))} ${label}" data-point="${esc(point.label)} · ${esc(fmt(point.value))} ${label}"><title>${esc(point.label)} · ${esc(fmt(point.value))}</title></circle>`;
  }).join('');
  return `<section class="panel">${header}<div class="chart"><svg viewBox="0 0 ${width} 153" aria-label="${title}"><defs><linearGradient id="chart-fill" x2="0" y2="1"><stop stop-color="#7bd3a0" stop-opacity=".48"/><stop offset="1" stop-color="#b7ecc6" stop-opacity=".05"/></linearGradient></defs>${[1,.75,.5,.25,0].map(ratio => { const y=bottom-(bottom-top)*ratio; return `<line class="gridline" x1="${left}" x2="${right}" y1="${y}" y2="${y}"/><text x="0" y="${y+4}">${fmt(Math.round(max*ratio))}</text>`; }).join('')}<path d="${path} L${coords.at(-1)[0]} ${bottom} L${left} ${bottom} Z" fill="url(#chart-fill)"/><path d="${path}" fill="none" stroke="#20855a" stroke-width="2.4" stroke-linejoin="round"/>${points.map((point,index) => index%step === 0 || index === points.length-1 ? `<text x="${coords[index][0]}" y="148" text-anchor="${index === points.length-1 ? 'end' : 'start'}">${esc(point.label)}</text>` : '').join('')}${marks}</svg><div class="chart-legend" aria-live="polite"><span></span><span data-chart-label="${label}">${label}</span></div></div></section>`;
}

function channels(website) {
  return `<section class="panel">${panelHead('Acquisition channels','Share of website sessions')}${website?.channels && typeof website.channels === 'object' ? `<div class="channels">${Object.entries(channelMeta).filter(([key]) => key in website.channels).map(([key,[glyph,color,label]]) => `<div class="channel">${tile(glyph,color)}<div><strong>${label}</strong><small>${key === 'social' ? 'Instagram and other social' : 'Website acquisition'}</small></div><span>${esc(website.channels[key])}%</span></div>`).join('')}</div>` : noData('website')}</section>`;
}

function features(rows,compact = false) {
  const actions = compact ? '<a href="#fleet" class="button link">View operations '+icon('arrow')+'</a>' : `<label class="control"><select id="feature-filter" aria-label="Filter fleet features">${['All','On track','Needs attention'].map(value => `<option ${featureFilter === value ? 'selected' : ''}>${value}</option>`).join('')}</select></label>`;
  const visible = (rows || []).filter(row => compact || featureFilter === 'All' || row.status === featureFilter);
  return `<section class="panel">${panelHead('Feature monitoring','Operational signals across your fleet services',actions)}${visible.length ? `<div class="table-wrap"><table class="data-table"><thead><tr><th scope="col">Feature</th><th scope="col">Metric</th><th scope="col">Value</th><th scope="col">Status</th><th scope="col"><span class="muted">Details</span></th></tr></thead><tbody>${visible.map(row => `<tr><td><span class="feature-name">${tile(row.id,row.id === 'fuel' ? 'yellow' : row.id === 'gps' ? 'blue' : '')}${esc(row.name)}</span></td><td class="muted">${esc(row.metric)}</td><td><strong>${esc(fmt(row.value))}</strong></td><td>${status(row.status,row.status === 'Needs attention' ? 'warn' : row.status === 'On track' ? '' : 'neutral')}</td><td><button class="button link table-action" data-action="feature" data-id="${esc(row.id)}" aria-label="Details for ${esc(row.name)}">${icon('chevron')}</button></td></tr>`).join('')}</tbody></table></div>` : noData('fleet')}</section>`;
}

const leadMatches = lead => {
  const query = leadSearch.trim().toLowerCase();
  const text = [lead.name,lead.company,lead.email,lead.phone,lead.interest,lead.note,lead.assignee].map(value => String(value ?? '').toLowerCase());
  return (leadFilter === 'All' || lead.status === leadFilter) && (leadSource === 'All' || lead.source === leadSource) && (!query || text.some(value => value.includes(query)));
};

function leadPage(leads) {
  const stages = ['New','Contacted','Qualified','Closed'];
  const rows = (leads || []).map(leadView);
  const visible = rows.filter(leadMatches);
  const sourceOptions = ['All',...new Set(rows.map(lead => lead.source).filter(Boolean))];
  const filters = `<div class="filters"><label class="control">${icon('search')}<input id="lead-search" type="search" placeholder="Search leads" aria-label="Search leads" value="${esc(leadSearch)}"></label><label class="control"><select id="lead-source" aria-label="Filter leads by source">${sourceOptions.map(value => `<option ${leadSource === value ? 'selected' : ''}>${esc(value)}</option>`).join('')}</select></label><label class="control"><select id="lead-filter" aria-label="Filter leads by stage">${['All',...stages].map(value => `<option ${leadFilter === value ? 'selected' : ''}>${value}</option>`).join('')}</select></label></div>`;
  const table = visible.length ? `<div class="table-wrap"><table class="data-table"><thead><tr><th scope="col">Contact</th><th scope="col">Interest</th><th scope="col">Source</th><th scope="col">Owner</th><th scope="col">Response</th><th scope="col">Stage</th><th scope="col">Details</th></tr></thead><tbody>${visible.map(lead => `<tr><td><strong>${esc(lead.name)}</strong><br><span class="muted">${esc(lead.company || lead.phone || lead.email)}</span></td><td>${esc(lead.interest)}</td><td>${esc(lead.source)}</td><td>${lead.assignee ? esc(lead.assignee) : '<span class="muted">Unassigned</span>'}</td><td>${responseCell(lead)}</td><td>${status(lead.status,lead.status === 'New' ? 'warn' : '')}</td><td><button class="button link table-action" data-action="lead" data-id="${esc(lead.id)}" aria-label="Details for ${esc(lead.name)}">${icon('chevron')}</button></td></tr>`).join('')}</tbody></table></div>` : rows.length ? '<p class="empty">No leads match these filters.</p>' : noData('leads');
  return `${pageHead('Leads','See who’s interested, what they need, and who needs a follow-up.',false,button('Export CSV','file','export-leads'))}${provenance('leads')}
    <div class="cards">${stages.map((stage,index) => `<button class="card" data-action="lead-stage" data-value="${stage}" aria-pressed="${leadFilter === stage}"><span class="card-title">${tile(index ? 'check' : 'people',['','blue','lavender','pink'][index])}${stage}</span><strong>${leads ? rows.filter(lead => lead.status === stage).length : '—'}</strong><small>${leadFilter === stage ? 'Showing this stage' : 'Filter this stage'}</small></button>`).join('')}</div>
    <section class="panel">${panelHead('Inquiry pipeline','Contact, source and service interest',filters)}${table}</section>`;
}

function inboxPage(messages) {
  const query = inboxSearch.trim().toLowerCase();
  const shown = (messages || []).filter(message => !query || [message.name,message.email,message.subject,message.body].some(value => String(value ?? '').toLowerCase().includes(query)));
  const active = messages?.find(message => message.id === selectedMessage);
  const list = shown.length ? shown.map(message => `<button type="button" class="message" data-action="message" data-id="${esc(message.id)}" aria-pressed="${message.id === selectedMessage}"><strong>${esc(message.name)} ${message.unread && !readMessages.has(message.id) ? status('Unread','warn') : ''}</strong><small>${esc(message.subject)}</small><small>${esc(message.date)}</small></button>`).join('') : messages?.length ? '<p class="empty">No messages match this search.</p>' : noData('inbox');
  const search = `<label class="control message-search">${icon('search')}<input id="inbox-search" type="search" placeholder="Search mail" aria-label="Search mail" value="${esc(inboxSearch)}"></label>`;
  const replying = sources.inbox?.state === 'live';
  const replyNote = replying && !can(2) ? '<small class="muted">Sending replies needs an admin account.</small>' : '';
  return `${pageHead('Email inbox','Read customer and dispatch messages and reply from the dashboard.')}${provenance('inbox')}<section class="panel inbox"><div class="messages">${search}${list}</div><div class="thread">${active ? `<h2>${esc(active.subject)}</h2><p class="muted">${esc(active.name)} &lt;${esc(active.email)}&gt; · ${esc(active.date)}</p><div class="thread-body">${esc(active.body)}</div>${(sentReplies[active.id] || []).map(reply => `<div class="reply-entry"><strong>${reply.live ? 'Reply accepted by mailbox' : 'Sample reply'}</strong><div class="thread-body">${esc(reply.body)}</div></div>`).join('')}<form class="reply" id="reply" data-id="${esc(active.id)}"><label for="reply-body">Your reply</label><textarea id="reply-body" required maxlength="12000">${esc(replyDrafts[active.id])}</textarea><button class="button primary" type="submit" ${replying && !can(2) ? 'disabled' : ''}>${icon('send')}${replying ? 'Send reply' : 'Try a sample reply'}</button> ${replyNote}</form>` : '<p class="empty">Select a message to read and reply.</p>'}</div></section>`;
}

function overview(data) {
  const {website,instagram,fleet,leads} = data;
  const exceptions = fleet?.features.filter(feature => feature.status === 'Needs attention');
  const newLeads = leads?.map(leadView).filter(lead => lead.status === 'New');
  const overdue = newLeads?.filter(isOverdue).length;
  return `${pageHead('Operations overview','Website performance, incoming inquiries and municipal fleet activity.',true)}${hero}<div class="cards">${card('Website sessions',website?.sessions,sourceHint('website'),'chart','','website',website?.traffic,website?.previous?.sessions)}${card('New leads',newLeads?.length,overdue ? `${overdue} past response target · ${sourceHint('leads')}` : sourceHint('leads'),'people','lavender','leads')}${card('Instagram reach',instagram?.reach,sourceHint('instagram'),'instagram','pink','instagram',instagram?.series,instagram?.previous?.reach)}${card('Fleet exceptions',exceptions?.length,sourceHint('fleet'),'alert','blue','fleet')}</div><div class="split">${chart('Website activity','Sessions and inquiries over the selected period',chartMetric === 'traffic' ? website?.traffic : website?.inquiries,chartMetric === 'traffic' ? 'Sessions' : 'Inquiries',true)}${channels(website)}</div>${features(fleet?.features,true)}`;
}

function websitePage(website) {
  const rows = website?.pages;
  return `${pageHead('Website analytics','See what brings people to the site and what they do next.',true,button('Export CSV','file','export-website'))}${provenance('website')}<div class="cards">${card('Sessions',website?.sessions,'Visits in this period','chart','','website',null,website?.previous?.sessions)}${card('Page views',website?.pageviews,'All page views','globe','blue','website',null,website?.previous?.pageviews)}${card('Website inquiries',website?.leads,'Completed inquiries','people','lavender','leads',null,website?.previous?.leads)}${card('Inquiry rate',website?.sessions && Number.isFinite(website.leads) ? `${(website.leads / website.sessions * 100).toFixed(2)}%` : null,'Inquiries per session','check','pink','leads')}</div><div class="split">${chart('Website activity','Sessions and inquiries over the selected period',chartMetric === 'traffic' ? website?.traffic : website?.inquiries,chartMetric === 'traffic' ? 'Sessions' : 'Inquiries',true)}${channels(website)}</div><div class="split"><section class="panel">${panelHead('Website pages','Views by page')}${rows?.length ? `<div class="table-wrap"><table class="data-table"><thead><tr><th scope="col">Page</th><th scope="col">Path</th><th scope="col">Views</th></tr></thead><tbody>${rows.map(row => `<tr><td>${sitePageLink(row)}</td><td class="muted">${esc(row.path)}</td><td><strong>${esc(fmt(row.views))}</strong></td></tr>`).join('')}</tbody></table></div>` : noData('website')}</section><section class="panel">${panelHead('Inquiry activity','Form submissions and contact clicks')}${website?.actions?.length ? `<div class="channels">${website.actions.map(row => `<div class="channel">${tile('arrow')}<strong>${esc(row.name)}</strong><span>${esc(fmt(row.count))}</span></div>`).join('')}</div>` : noData('website')}</section></div>`;
}

function fleetPage(fleet) {
  const wards = fleet?.wards;
  return `${pageHead('Fleet operations','Monitor collection, dispatch, fuel and plant reconciliation.',sources.fleet?.state === 'live')}${provenance('fleet')}<div class="stack">${wards?.length ? `<section class="panel">${panelHead('Ward stop board','Completed stops and missed collections')}<div class="ward-board">${wards.map(ward => `<div class="ward"><strong>${esc(ward.name)}</strong><p>${esc(fmt(ward.completed))} of ${esc(fmt(ward.scheduled))} completed</p><progress max="${Math.max(1,Number(ward.scheduled) || 0)}" value="${Math.max(0,Number(ward.completed) || 0)}" aria-label="Completed stops in ${esc(ward.name)}"></progress>${status(`${fmt(ward.missed)} missed`,ward.missed ? 'warn' : '')}</div>`).join('')}</div></section>` : ''}${features(fleet?.features)}</div>`;
}

function instagramPage(instagram) {
  return `${pageHead('Instagram reach','Follow account reach and engagement from Instagram Insights.',true)}${provenance('instagram')}<div class="cards">${card('Accounts reached',instagram?.reach,'Unique accounts in this period','instagram','pink','instagram',null,instagram?.previous?.reach)}${card('Engagements',instagram?.engagements,'Interactions in this period','people','lavender','instagram',null,instagram?.previous?.engagements)}${card('Followers',instagram?.followers,'Account total','people','','instagram')}${card('Profile visits',instagram?.profileVisits,'Visits in this period','globe','blue','instagram',null,instagram?.previous?.profileVisits)}</div>${chart('Reach over time','Reach by reporting interval',instagram?.series,'Accounts reached')}`;
}

const esgStreams = ['Wet','Dry','Plastic','Reject'];
const esgColors = {Wet:'#48a276',Dry:'#7bd3a0',Plastic:'#5b9bd5',Reject:'#b59a76'};
const esgNumber = value => Number.isFinite(Number(value)) ? Number(value) : 0;

function esgPage(fleet) {
  const esg = fleet?.esg, periods = esg?.periods?.filter(row => row && typeof row === 'object') || [];
  const header = pageHead('ESG & recovery','Tonnes recovered by waste stream, diversion from reject, and pending ESG exports.',false,periods.length ? button('Export CSV','file','export-esg') : '');
  if (!periods.length) return `${header}${provenance('fleet')}<section class="panel">${panelHead('Recovery by waste stream','Monthly tonnes')}<p class="empty">${sources.fleet?.state === 'error' ? 'The source is unavailable. Refresh to retry.' : 'ESG figures appear once your fleet system provides an esg.periods list.'}</p></section>`;
  const total = row => esgStreams.reduce((sum,stream) => sum + esgNumber(row[stream]),0);
  const totals = Object.fromEntries(esgStreams.map(stream => [stream,periods.reduce((sum,row) => sum + esgNumber(row[stream]),0)]));
  const all = esgStreams.reduce((sum,stream) => sum + totals[stream],0);
  const diverted = all - totals.Reject;
  const rate = row => total(row) ? `${((total(row) - esgNumber(row.Reject)) / total(row) * 100).toFixed(1)}%` : '—';
  const peak = Math.max(1,...periods.map(total));
  const slot = 560 / periods.length, barWidth = Math.min(48,slot * .6);
  const bars = periods.map((row,index) => {
    let y = 128;
    const x = 30 + index * slot + (slot - barWidth) / 2;
    const stack = esgStreams.map(stream => { const height = 112 * esgNumber(row[stream]) / peak; y -= height; return `<rect x="${x.toFixed(1)}" y="${y.toFixed(1)}" width="${barWidth.toFixed(1)}" height="${height.toFixed(1)}" fill="${esgColors[stream]}"><title>${esc(row.label)} · ${stream}: ${fmt(esgNumber(row[stream]))} t</title></rect>`; }).join('');
    return `${stack}<text x="${(x + barWidth / 2).toFixed(1)}" y="146" text-anchor="middle">${esc(row.label)}</text>`;
  }).join('');
  const legend = `<div class="chart-legend esg-legend">${esgStreams.map(stream => `<span><i style="background:${esgColors[stream]}"></i>${stream}</span>`).join('')}</div>`;
  return `${header}${provenance('fleet')}<div class="cards">${card('Tonnes recovered',Math.round(diverted),'Wet, dry and plastic','bolt','','esg')}${card('Recovery rate',all ? `${(diverted / all * 100).toFixed(1)}%` : null,'Share not sent to reject','check','pink','esg')}${card('Reject',Math.round(totals.Reject),'Tonnes in this period','alert','blue','esg')}${card('Pending ESG exports',esg.pendingExports,'Reports waiting to be filed','file','lavender','esg')}</div>
  <section class="panel">${panelHead('Recovery by waste stream','Tonnes per month')}<div class="chart"><svg viewBox="0 0 590 153" role="img" aria-label="Stacked bars of tonnes by waste stream per month"><line class="gridline" x1="30" x2="590" y1="128" y2="128"/>${bars}</svg>${legend}</div></section>
  <section class="panel">${panelHead('Monthly detail','Tonnes by stream')}<div class="table-wrap"><table class="data-table"><thead><tr><th scope="col">Period</th>${esgStreams.map(stream => `<th scope="col">${stream}</th>`).join('')}<th scope="col">Total</th><th scope="col">Recovery rate</th></tr></thead><tbody>${periods.map(row => `<tr><td><strong>${esc(row.label)}</strong></td>${esgStreams.map(stream => `<td>${esc(fmt(esgNumber(row[stream])))}</td>`).join('')}<td><strong>${esc(fmt(total(row)))}</strong></td><td>${rate(row)}</td></tr>`).join('')}</tbody></table></div></section>`;
}

function activityPage() {
  const head = pageHead('Activity log','Sign-in failures and every change made through the dashboard, newest first.',false,auditRows?.length ? button('Export CSV','file','export-activity') : '');
  if (!can(2)) return `${head}<p class="empty">The activity log is available to admin accounts.</p>`;
  if (auditRows === null && !auditLoading) loadAudit();
  if (auditRows === null) return `${head}<p class="source-note">Loading…</p>`;
  const rows = auditRows.map(entry => `<tr><td class="muted">${esc(new Date(entry.at).toLocaleString('en-US',{dateStyle:'medium',timeStyle:'medium'}))}</td><td><strong>${esc(entry.user)}</strong></td><td class="muted">${esc(entry.ip)}</td><td>${esc(entry.method)} ${esc(entry.path)}</td><td>${status(String(entry.status),entry.status >= 400 ? 'warn' : '')}</td></tr>`).join('');
  return `${head}<section class="panel">${panelHead('Recent activity','The last 500 events. Set ARECOSYS_AUDIT_FILE to keep them across restarts.')}${rows ? `<div class="table-wrap"><table class="data-table"><thead><tr><th scope="col">Time</th><th scope="col">User</th><th scope="col">Address</th><th scope="col">Action</th><th scope="col">Result</th></tr></thead><tbody>${rows}</tbody></table></div>` : '<p class="empty">No activity recorded yet.</p>'}</section>`;
}

async function loadAudit() {
  auditLoading = true;
  try { auditRows = location.protocol === 'file:' ? [] : await api('audit'); } catch { auditRows = []; }
  auditLoading = false;
  if (pageName() === 'activity') render();
}

function connections() {
  const guidance = {website:'Site traffic, signups and contact actions',fleet:'WasteFlow ERP and fleet telemetry',instagram:'Meta professional account insights',leads:'Contact forms, WhatsApp and qualified inquiries',inbox:'Gmail inbox and threaded replies'};
  return `${pageHead('Data connections','Check which services are linked to this dashboard.')}<section class="panel">${panelHead('Sources','Connection status reflects the most recent refresh.')}
    <div class="table-wrap"><table class="data-table"><thead><tr><th scope="col">Source</th><th scope="col">Data</th><th scope="col">Status</th><th scope="col">Last refreshed</th></tr></thead><tbody>${Object.entries(sourceNames).map(([name,label]) => {const state = sources[name]?.state;return `<tr><td><strong>${label}</strong></td><td class="muted">${guidance[name]}</td><td>${status(state === 'live' ? 'Connected' : state === 'error' ? 'Unavailable' : 'Sample data',state === 'error' ? 'warn' : state === 'live' ? '' : 'neutral')}</td><td class="muted">${updatedAt(name) || '—'}</td></tr>`;}).join('')}</tbody></table></div></section>`;
}

function render() {
  const current = pageName();
  const data = dashboard();
  document.querySelector('#breadcrumb').textContent = (marketingPages.includes(current) ? 'Marketing / ' : '') + pages[current];
  const group = document.querySelector('#marketing-group');
  group.classList.toggle('has-current',marketingPages.includes(current));
  if (marketingPages.includes(current)) setMarketingOpen(true);
  document.querySelectorAll('[data-page]').forEach(link => {
    if (link.dataset.page === current) link.setAttribute('aria-current','page');
    else link.removeAttribute('aria-current');
  });
  document.querySelector('[data-page="activity"]').hidden = !can(2);
  const states = Object.values(sources).map(source => source.state);
  const mode = document.querySelector('#mode');
  mode.classList.toggle('live',states.includes('live'));
  mode.lastChild.textContent = states.length && states.every(state => state === 'live') ? ' Connected sources' : states.includes('live') ? ' Connected + preview' : states.includes('error') ? ' Source issue · preview' : ' Preview data';
  const counts = {'unread':data.inbox?.filter(message => message.unread && !readMessages.has(message.id)).length,'lead-count':data.leads?.filter(lead => leadView(lead).status === 'New').length};
  for (const [id,count] of Object.entries(counts)) {
    const badge = document.getElementById(id);
    badge.hidden = !count;
    badge.textContent = count || '';
  }
  main.innerHTML = ({overview:() => overview(data),website:() => websitePage(data.website),instagram:() => instagramPage(data.instagram),fleet:() => fleetPage(data.fleet),leads:() => leadPage(data.leads),inbox:() => inboxPage(data.inbox),sources:connections,design:designPage,esg:() => esgPage(data.fleet),activity:activityPage})[current]();
  if (current === 'design') drawDesign();
}

function notify(message,isError = false) {
  const notice = document.querySelector('#notice');
  notice.textContent = message;
  notice.classList.toggle('error',isError);
  notice.hidden = false;
  clearTimeout(noticeTimer);
  noticeTimer = setTimeout(() => { notice.hidden = true; },4300);
}

async function api(path,options = {}) {
  const response = await fetch(`/api/${path}`,{cache:'no-store',...options});
  if (!response.ok) throw new Error(`Request failed (${response.status})`);
  return response.status === 204 ? null : response.json();
}

function validSource(name, source) {
  if (!source || !['live','demo','error'].includes(source.state)) return {state:'error'};
  if (source.state !== 'live') return source;
  const {data} = source;
  const arrays = {website:['pages','actions','traffic','inquiries'],fleet:['features','wards'],instagram:['series']};
  const validArray = value => Array.isArray(value) && value.every(item => item && typeof item === 'object');
  if (name === 'inbox' || name === 'leads') return validArray(data) ? source : {state:'error'};
  const valid = data && typeof data === 'object' && !Array.isArray(data) && arrays[name].every(key => data[key] == null || validArray(data[key])) && (name !== 'fleet' || validArray(data.features) && (data.drilldowns == null || typeof data.drilldowns === 'object' && !Array.isArray(data.drilldowns) && Object.values(data.drilldowns).every(validArray))) && (data.previous == null || (typeof data.previous === 'object' && !Array.isArray(data.previous))) && (name !== 'fleet' || data.esg == null || Array.isArray(data.esg.periods));
  return valid ? source : {state:'error'};
}

async function refresh(report = false) {
  if (location.protocol === 'file:') {
    render();
    if (report) notify('Preview data refreshed.');
    return;
  }
  if (pageName() === 'design') loadDesign(true);
  if (pageName() === 'activity') loadAudit();
  const request = ++requestNumber;
  loading = true;
  const refreshButton = main.querySelector('[data-action="refresh"]');
  if (refreshButton) { refreshButton.disabled = true; refreshButton.lastChild.textContent = 'Refreshing…'; }
  try {
    const payload = await api(`dashboard?days=${days}`);
    if (request !== requestNumber) return;
    for (const name of Object.keys(sourceNames)) sources[name] = validSource(name,payload[name]);
    const stamp = new Date();
    for (const name of Object.keys(sourceNames)) refreshedAt[name] = stamp;
    const hasErrors = Object.values(sources).some(source => source.state === 'error');
    if (report) notify(hasErrors ? 'Some sources could not be refreshed.' : 'Dashboard refreshed.',hasErrors);
  } catch {
    if (request !== requestNumber) return;
    for (const source of Object.values(sources)) if (source.state === 'live') { source.state = 'error'; source.data = null; }
    if (report) notify('Could not reach the data server.',true);
  } finally {
    if (request === requestNumber) {
      loading = false;
      if (report && refreshButton && main.contains(refreshButton)) rerenderFrom('[data-action="refresh"]');
      else render();
    }
  }
}

function drilldown(rows) {
  if (!Array.isArray(rows) || !rows.length) return '';
  const shown = rows.slice(0,50);
  const columns = [...new Set(shown.flatMap(row => Object.keys(row)))].slice(0,6);
  return `<h3 class="drill-title">Affected records</h3><div class="table-wrap"><table class="data-table"><thead><tr>${columns.map(column => `<th scope="col">${esc(column)}</th>`).join('')}</tr></thead><tbody>${shown.map(row => `<tr>${columns.map(column => `<td>${esc(fmt(row[column]))}</td>`).join('')}</tr>`).join('')}</tbody></table></div>${rows.length > 50 ? `<p class="footnote">Showing 50 of ${rows.length} records.</p>` : ''}`;
}

function detail(title,description,fields,extra = '') {
  dialog.innerHTML = `<div class="dialog-head"><h2 id="detail-title">${esc(title)}</h2><button type="button" class="button" data-action="close" aria-label="Close details">${icon('close')}</button></div><p class="footnote">${esc(description)}</p><dl class="detail-list">${Object.entries(fields).filter(([,value]) => value != null && value !== '').map(([label,value]) => `<div><dt>${esc(label)}</dt><dd>${esc(fmt(value))}</dd></div>`).join('')}</dl>${extra}`;
  dialog.classList.toggle('wide',extra.includes('drill-title'));
  dialog.showModal();
}

const rerenderFrom = selector => { render(); main.querySelector(selector)?.focus(); };
const leadTimeline = row => {
  const entries = [...(row.activity || [])].reverse();
  const items = entries.map(entry => `<li><strong>${esc(entry.by)}</strong> <span class="muted">${esc(new Date(entry.at).toLocaleString('en-US',{dateStyle:'medium',timeStyle:'short'}))}</span><br>${esc(entry.text)}</li>`).join('');
  return `<h3 class="drill-title">Timeline</h3><ul class="timeline">${items || '<li class="muted">No activity yet.</li>'}</ul><form class="reply" id="lead-note" data-id="${esc(row.id)}" data-live="${sources.leads?.state === 'live'}"><label class="control">Add a note <input id="lead-note-text" maxlength="1000" required></label> <button class="button" ${can(1) ? '' : 'disabled'}>Add note</button></form>`;
};

function openLead(id) {
  const row = dashboard().leads?.find(lead => lead.id === id);
  if (!row) return;
  const live = sources.leads?.state === 'live';
  const view = leadView(row);
  const due = dueAt(row.created);
  const message = dashboard().inbox?.find(item => item.email && item.email.toLowerCase() === String(row.email).toLowerCase());
  const related = message && live === (sources.inbox?.state === 'live') ? `<button type="button" class="button link" data-action="lead-email" data-id="${esc(message.id)}">${icon('mail')}View email</button>` : '';
  detail(row.name,row.note,{Company:row.company,Email:row.email,Phone:row.phone,Channel:row.source,Interest:row.interest,Received:row.created,'Response due':row.responded || !due ? null : due.toLocaleString(),'First response':row.responded,Campaign:row.campaign},`<form class="reply" id="lead-stage" data-id="${esc(id)}" data-live="${live}"><label class="control">Owner <input id="lead-owner" maxlength="80" value="${esc(view.assignee)}" placeholder="Unassigned"></label> <label class="control">Stage <select id="lead-stage-value">${['New','Contacted','Qualified','Closed'].map(value => `<option ${view.status === value ? 'selected' : ''}>${value}</option>`).join('')}</select></label> <button class="button primary" ${can(1) ? '' : 'disabled'}>Save stage</button> ${related}</form>${leadTimeline(row)}`);
}

document.addEventListener('click',event => {
  const target = event.target.closest('[data-action]');
  if (!target) return;
  const {action,id,value} = target.dataset;
  const data = dashboard();
  if (action === 'refresh') refresh(true);
  if (action === 'export-leads') exportLeads();
  if (action === 'export-website') exportWebsite();
  if (action === 'export-esg') exportEsg();
  if (action === 'export-activity') exportActivity();
  if (action === 'close') dialog.close();
  if (action === 'chart') { chartMetric = value; rerenderFrom(`[data-action="chart"][data-value="${value}"]`); }
  if (action === 'lead-stage') { leadFilter = leadFilter === value ? 'All' : value; rerenderFrom(`[data-action="lead-stage"][data-value="${value}"]`); }
  if (action === 'message') { selectedMessage = id; readMessages.add(id); rerenderFrom(`[data-action="message"][aria-pressed="true"]`); }
  if (action === 'feature') {
    const row = data.fleet?.features.find(feature => feature.id === id);
    if (row) detail(row.name,row.status,{[row.metric]:row.value,...row.details},drilldown(data.fleet?.drilldowns?.[id]));
  }
  if (action === 'lead') openLead(id);
  if (action === 'lead-email') {
    selectedMessage = id;
    readMessages.add(id);
    dialog.close();
    location.hash = 'inbox';
  }
});

const stamp = () => new Date().toISOString().slice(0,10);
function exportLeads() {
  const rows = (dashboard().leads || []).map(leadView).filter(leadMatches);
  downloadCsv(`arecosys-leads-${stamp()}.csv`,[['Name','Company','Email','Phone','Interest','Source','Owner','Stage','Received','First response'],...rows.map(lead => [lead.name,lead.company,lead.email,lead.phone,lead.interest,lead.source,lead.assignee,lead.status,lead.created,lead.responded])]);
}
function exportWebsite() {
  const website = dashboard().website || {};
  downloadCsv(`arecosys-website-${days}d-${stamp()}.csv`,[['Page','Path','Views'],...(website.pages || []).map(row => [row.name,row.path,row.views]),[],['Date','Sessions','Inquiries'],...(website.traffic || []).map((row,index) => [row.label,row.value,website.inquiries?.[index]?.value ?? ''])]);
}
function exportEsg() {
  const periods = dashboard().fleet?.esg?.periods || [];
  downloadCsv(`arecosys-recovery-${stamp()}.csv`,[['Period',...esgStreams,'Total'],...periods.map(row => [row.label,...esgStreams.map(stream => esgNumber(row[stream])),esgStreams.reduce((sum,stream) => sum + esgNumber(row[stream]),0)])]);
}
function exportActivity() {
  downloadCsv(`arecosys-activity-${stamp()}.csv`,[['Time','User','Address','Method','Path','Status'],...(auditRows || []).map(entry => [entry.at,entry.user,entry.ip,entry.method,entry.path,entry.status])]);
}

document.addEventListener('submit',async event => {
  const form = event.target;
  if (form.id !== 'lead-note') return;
  event.preventDefault();
  const text = form.querySelector('input').value.trim();
  const id = form.dataset.id;
  if (!text) return;
  try {
    if (form.dataset.live === 'true') await api(`leads/${encodeURIComponent(id)}`,{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({note:text})});
    const row = dashboard().leads?.find(lead => lead.id === id);
    if (row) (row.activity ||= []).push({at:new Date().toISOString(),by:session.name || 'You',text});
    openLead(id);
    notify('Note added.');
  } catch { notify('The note could not be saved.',true); }
});

main.addEventListener('change',event => {
  const {id,value} = event.target;
  if (id === 'range') {
    days = Number(value);
    for (const name of ['website','fleet','instagram']) if (sources[name]?.state === 'live') sources[name].data = null;
    rerenderFrom('#range');
    refresh().then(() => main.querySelector('#range')?.focus());
  }
  if (id === 'feature-filter') { featureFilter = value; rerenderFrom('#feature-filter'); }
  if (id === 'lead-filter') { leadFilter = value; rerenderFrom('#lead-filter'); }
  if (id === 'lead-source') { leadSource = value; rerenderFrom('#lead-source'); }
});
main.addEventListener('input',event => {
  const {id,value} = event.target;
  if (id === 'reply-body') replyDrafts[selectedMessage] = value;
  if (id === 'lead-search' || id === 'inbox-search') {
    if (id === 'lead-search') leadSearch = value; else inboxSearch = value;
    render();
    const field = document.getElementById(id);
    field.focus();
    field.setSelectionRange(value.length,value.length);
  }
});
document.addEventListener('submit',async event => {
  const form = event.target;
  if (!['reply','lead-stage'].includes(form.id)) return;
  event.preventDefault();
  const {id} = form.dataset;
  const submit = form.querySelector('button[type="submit"], button:not([type])');
  if (submit.disabled || form.id === 'reply' && pendingReplies.has(id)) return;
  submit.disabled = true;
  try {
    if (form.id === 'lead-stage') {
      const stage = form.querySelector('select').value;
      const owner = form.querySelector('input').value.trim();
      if (form.dataset.live === 'true') {
        await api(`leads/${encodeURIComponent(id)}`,{method:'PATCH',headers:{'Content-Type':'application/json'},body:JSON.stringify({status:stage,assignee:owner})});
        const lead = sources.leads.data?.find(row => row.id === id);
        if (lead) {
          lead.status = stage;
          lead.assignee = owner;
          if (stage !== 'New' && !lead.responded) lead.responded = new Date().toISOString();
        }
      } else { demoStages[id] = stage; demoOwners[id] = owner; }
      dialog.close();
      notify('Lead stage updated.');
    } else {
      const body = form.querySelector('textarea').value.trim();
      if (!body) { notify('Enter a reply before sending.',true); return; }
      const live = sources.inbox?.state === 'live';
      if (live) {
        pendingReplies.add(id);
        await api(`inbox/${encodeURIComponent(id)}/reply`,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({body})});
      }
      (sentReplies[id] ||= []).push({body,live});
      delete replyDrafts[id];
      notify(live ? 'Gmail accepted your reply.' : 'Sample reply added to the preview.');
    }
    render();
  } catch {
    notify('The request could not be confirmed. Check the source before retrying.',true);
  } finally {
    if (form.id === 'reply') pendingReplies.delete(id);
    submit.disabled = false;
  }
});

for (const event of ['focusin','pointerover','pointerout','focusout']) {
  main.addEventListener(event,({target}) => {
    if (!target.dataset.point) return;
    const label = target.closest('.chart').querySelector('[data-chart-label]');
    label.textContent = event === 'pointerout' || event === 'focusout' ? label.dataset.chartLabel : target.dataset.point;
  });
}
window.addEventListener('hashchange',() => { render(); main.focus(); window.scrollTo(0,0); });
document.querySelectorAll('[data-icon]').forEach(node => { node.innerHTML = icon(node.dataset.icon); });
async function loadSession() {
  if (location.protocol === 'file:') return;
  try {
    const me = await api('me');
    if (!me?.role) return;
    Object.assign(session,{name:me.name,role:me.role});
    document.querySelector('#user-name').textContent = me.name;
    document.querySelector('#user-role').textContent = me.role[0].toUpperCase() + me.role.slice(1);
    document.querySelector('.workspace-avatar').textContent = me.name[0]?.toUpperCase() ?? 'A';
    render();
  } catch { /* The dashboard still works read-only without a session. */ }
}

let marketingOpen = true;
try { marketingOpen = localStorage.getItem('marketing-open') !== 'false'; } catch { /* Defaults to open. */ }
setMarketingOpen(marketingOpen);
document.querySelector('#marketing-group .nav-toggle').addEventListener('click',event => {
  setMarketingOpen(event.currentTarget.getAttribute('aria-expanded') !== 'true',true);
});

const themeButton = document.querySelector('#theme');
const applyTheme = theme => {
  document.documentElement.dataset.theme = theme;
  themeButton.setAttribute('aria-checked',String(theme === 'dark'));
};
let savedTheme = '';
try { savedTheme = localStorage.getItem('theme') || ''; } catch { /* Storage can be blocked. */ }
applyTheme(savedTheme || (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'));
themeButton.addEventListener('click',() => {
  const next = document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark';
  applyTheme(next);
  try { localStorage.setItem('theme',next); } catch { /* Not remembered. */ }
});

render();
loadSession();
refresh();
if (location.protocol !== 'file:') setInterval(async () => {
  if (pageName() !== 'fleet' || document.hidden || dialog.open || main.contains(document.activeElement) && document.activeElement !== main || loading || sources.fleet?.state === 'demo' || !sources.fleet) return;
  const request = requestNumber;
  const currentDays = days;
  let next = {state:'error'};
  try { next = validSource('fleet',await api(`dashboard?source=fleet&days=${days}`)); } catch { /* Display the connection error below. */ }
  if (request === requestNumber && currentDays === days && pageName() === 'fleet' && !dialog.open && (!main.contains(document.activeElement) || document.activeElement === main)) {
    sources.fleet = next;
    render();
  }
},30000);
