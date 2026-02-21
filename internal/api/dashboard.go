package api

// DashboardHTML returns the full HTML page for the live dashboard
func DashboardHTML() string {
   return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>cloud dashboard</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,Helvetica,Arial,sans-serif;background:#0f1117;color:#c9d1d9;font-size:14px}
a{color:#58a6ff;text-decoration:none}
.wrap{max-width:1280px;margin:0 auto;padding:20px}
header{display:flex;justify-content:space-between;align-items:center;margin-bottom:24px}
header h1{font-size:20px;font-weight:600;color:#e6edf3}
header span{font-size:12px;color:#8b949e}
.stats{display:grid;grid-template-columns:repeat(5,1fr);gap:12px;margin-bottom:24px}
.stat-card{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px;text-align:center}
.stat-card .label{font-size:11px;text-transform:uppercase;letter-spacing:.5px;color:#8b949e;margin-bottom:6px}
.stat-card .value{font-size:28px;font-weight:700;color:#e6edf3}
.submit-bar{background:#161b22;border:1px solid #30363d;border-radius:8px;padding:16px;margin-bottom:24px;display:flex;gap:12px;align-items:center;flex-wrap:wrap}
.submit-bar input[type=text]{flex:1;min-width:200px;padding:8px 12px;background:#0d1117;border:1px solid #30363d;border-radius:6px;color:#c9d1d9;font-size:14px;outline:none}
.submit-bar input[type=text]:focus{border-color:#58a6ff}
.submit-bar select{padding:8px 12px;background:#0d1117;border:1px solid #30363d;border-radius:6px;color:#c9d1d9;font-size:14px;outline:none;cursor:pointer}
.submit-bar button{padding:8px 20px;background:#238636;border:none;border-radius:6px;color:#fff;font-size:14px;font-weight:600;cursor:pointer}
.submit-bar button:hover{background:#2ea043}
.tables{display:grid;grid-template-columns:2fr 1fr;gap:16px}
@media(max-width:900px){.tables{grid-template-columns:1fr}.stats{grid-template-columns:repeat(3,1fr)}}
.panel{background:#161b22;border:1px solid #30363d;border-radius:8px;overflow:hidden}
.panel h2{font-size:14px;font-weight:600;color:#e6edf3;padding:12px 16px;border-bottom:1px solid #30363d;display:flex;justify-content:space-between;align-items:center}
.panel h2 .count{background:#30363d;color:#8b949e;font-size:11px;padding:2px 8px;border-radius:10px}
table{width:100%;border-collapse:collapse}
th{text-align:left;font-size:11px;text-transform:uppercase;letter-spacing:.5px;color:#8b949e;padding:8px 16px;border-bottom:1px solid #30363d}
td{padding:8px 16px;border-bottom:1px solid #21262d;font-size:13px;max-width:200px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
tr:hover{background:#1c2129}
.badge{display:inline-block;padding:2px 8px;border-radius:10px;font-size:11px;font-weight:600}
.badge-high{background:#da363340;color:#f85149}
.badge-normal{background:#30363d;color:#8b949e}
.badge-low{background:#1f6feb30;color:#58a6ff}
.status-completed{color:#3fb950}
.status-running,.status-queued{color:#d29922}
.status-failed{color:#f85149}
.status-pending{color:#8b949e}
.status-cancelled{color:#8b949e}
.worker-idle{color:#3fb950}
.worker-busy{color:#d29922}
.empty{padding:32px;text-align:center;color:#484f58}
</style>
</head>
<body>
<div class="wrap">
<header>
<h1>cloud dashboard</h1>
<span id="updated"></span>
</header>

<div class="stats">
<div class="stat-card"><div class="label">queue depth</div><div class="value" id="s-queue">-</div></div>
<div class="stat-card"><div class="label">workers</div><div class="value" id="s-workers">-</div></div>
<div class="stat-card"><div class="label">total jobs</div><div class="value" id="s-jobs">-</div></div>
<div class="stat-card"><div class="label">success rate</div><div class="value" id="s-rate">-</div></div>
<div class="stat-card"><div class="label">uptime</div><div class="value" id="s-uptime">-</div></div>
</div>

<div class="submit-bar">
<input type="text" id="f-payload" placeholder="payload (e.g. hello world)">
<select id="f-priority">
<option value="0">high priority</option>
<option value="1" selected>normal priority</option>
<option value="2">low priority</option>
</select>
<button onclick="submitJob()">submit job</button>
<span id="f-msg" style="font-size:12px;color:#3fb950"></span>
</div>

<div class="tables">
<div class="panel">
<h2>jobs <span class="count" id="job-count">0</span></h2>
<div id="job-table-wrap">
<table><thead><tr><th>id</th><th>payload</th><th>priority</th><th>status</th><th>result</th><th>created</th></tr></thead><tbody id="job-rows"></tbody></table>
</div>
</div>
<div class="panel">
<h2>workers <span class="count" id="worker-count">0</span></h2>
<div id="worker-table-wrap">
<table><thead><tr><th>id</th><th>status</th><th>current job</th><th>last heartbeat</th></tr></thead><tbody id="worker-rows"></tbody></table>
</div>
</div>
</div>
</div>

<script>
const priorityLabel={0:'<span class="badge badge-high">high</span>',1:'<span class="badge badge-normal">normal</span>',2:'<span class="badge badge-low">low</span>'};

function statusClass(s){return 'status-'+(s||'pending')}
function workerClass(s){return 'worker-'+(s||'idle')}
function ago(ts){
  if(!ts)return'-';
  const d=new Date(ts),now=new Date(),sec=Math.floor((now-d)/1000);
  if(sec<60)return sec+'s ago';
  if(sec<3600)return Math.floor(sec/60)+'m ago';
  return Math.floor(sec/3600)+'h ago';
}
function fmtUptime(sec){
  if(sec<60)return Math.floor(sec)+'s';
  if(sec<3600)return Math.floor(sec/60)+'m';
  const h=Math.floor(sec/3600),m=Math.floor((sec%3600)/60);
  return h+'h '+m+'m';
}

async function fetchStats(){
  try{
    const r=await fetch('/stats');const d=await r.json();
    document.getElementById('s-queue').textContent=d.queue_depth;
    document.getElementById('s-workers').textContent=d.workers;
    document.getElementById('s-jobs').textContent=d.jobs_total;
    document.getElementById('s-rate').textContent=d.success_rate_pct.toFixed(0)+'%';
    document.getElementById('s-uptime').textContent=fmtUptime(d.uptime_seconds);
  }catch(e){}
}

async function fetchJobs(){
  try{
    const r=await fetch('/jobs?limit=200');const d=await r.json();
    const jobs=d.jobs||[];
    document.getElementById('job-count').textContent=d.total||0;
    const tbody=document.getElementById('job-rows');
    if(!jobs.length){tbody.innerHTML='<tr><td colspan="6" class="empty">no jobs yet</td></tr>';return;}
    jobs.sort((a,b)=>(a.priority-b.priority)||new Date(b.created_at)-new Date(a.created_at));
    tbody.innerHTML=jobs.map(j=>'<tr>'+
      '<td title="'+j.id+'">'+j.id.slice(0,10)+'</td>'+
      '<td title="'+esc(j.payload)+'">'+esc(j.payload)+'</td>'+
      '<td>'+(priorityLabel[j.priority]||priorityLabel[1])+'</td>'+
      '<td class="'+statusClass(j.status)+'">'+j.status+'</td>'+
      '<td title="'+(esc(j.result||j.error||''))+'">'+esc(j.result||j.error||'-')+'</td>'+
      '<td>'+ago(j.created_at)+'</td>'+
    '</tr>').join('');
  }catch(e){}
}

async function fetchWorkers(){
  try{
    const r=await fetch('/workers');const d=await r.json();
    const workers=d||[];
    document.getElementById('worker-count').textContent=workers.length;
    const tbody=document.getElementById('worker-rows');
    if(!workers.length){tbody.innerHTML='<tr><td colspan="4" class="empty">no workers registered</td></tr>';return;}
    tbody.innerHTML=workers.map(w=>'<tr>'+
      '<td title="'+w.id+'">'+w.id.slice(0,16)+'</td>'+
      '<td class="'+workerClass(w.status)+'">'+w.status+'</td>'+
      '<td>'+(w.current_job_id||'-')+'</td>'+
      '<td>'+ago(w.last_heartbeat)+'</td>'+
    '</tr>').join('');
  }catch(e){}
}

function esc(s){if(!s)return'';return s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');}

async function submitJob(){
  const payload=document.getElementById('f-payload').value.trim();
  if(!payload)return;
  const priority=parseInt(document.getElementById('f-priority').value);
  try{
    const r=await fetch('/jobs',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({payload,priority})});
    const d=await r.json();
    document.getElementById('f-msg').textContent='submitted: '+d.id;
    document.getElementById('f-payload').value='';
    setTimeout(()=>{document.getElementById('f-msg').textContent='';},3000);
    refresh();
  }catch(e){document.getElementById('f-msg').textContent='error';document.getElementById('f-msg').style.color='#f85149';}
}

function refresh(){fetchStats();fetchJobs();fetchWorkers();document.getElementById('updated').textContent='updated '+new Date().toLocaleTimeString();}
refresh();
setInterval(refresh,2000);
</script>
</body>
</html>`
}
