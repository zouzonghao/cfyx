package modes

import (
	"cf-optimizer/cloudflare"
	"cf-optimizer/config"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
)

type groupStatus struct {
	Name     string   `json:"name"`
	Hosts    []string `json:"hosts"`
	ManualIP string   `json:"manualIP"`
	BestIP   string   `json:"bestIP"`
}

type manualSettingsRequest struct {
	ManualMode bool              `json:"manualMode"`
	ManualIPs  map[string]string `json:"manualIPs"`
}

type manualSettingsResponse struct {
	ManualMode bool          `json:"manualMode"`
	Groups     []groupStatus `json:"groups"`
}

func ConfigPageHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, configPage)
}

func ManualSettingsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeManualSettings(w, http.StatusOK)
	case http.MethodPut:
		updateManualSettings(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func updateManualSettings(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	var request manualSettingsRequest
	if err := decoder.Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("invalid request: %v", err), http.StatusBadRequest)
		return
	}
	if err := config.UpdateManualSettings(request.ManualMode, request.ManualIPs); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if request.ManualMode {
		applyManualIPs(request.ManualIPs)
	}
	writeManualSettings(w, http.StatusOK)
}

func applyManualIPs(manualIPs map[string]string) {
	bestIPsMu.Lock()
	for group, ip := range manualIPs {
		bestIPsByGroup[group] = ip
	}
	bestIPsMu.Unlock()

	for host, hostInfo := range config.Current.HostMap {
		ip := manualIPs[hostInfo.Group]
		if err := cloudflare.UpdateDNSRecord(config.Current.Cloudflare.ZoneID, hostInfo.ID, config.Current.Cloudflare.APIToken, host, ip); err != nil {
			log.Printf("Manual mode: Error updating DNS for %s: %v", host, err)
		}
	}
}

func LoadManualIPs() {
	manualMode, manualIPs := config.ManualSettings()
	if !manualMode {
		return
	}
	bestIPsMu.Lock()
	for group, ip := range manualIPs {
		bestIPsByGroup[group] = ip
	}
	bestIPsMu.Unlock()
}

func writeManualSettings(w http.ResponseWriter, status int) {
	manualMode, manualIPs := config.ManualSettings()
	groups := config.GroupNames()
	hostsByGroup := make(map[string][]string, len(groups))
	for host, hostInfo := range config.Current.HostMap {
		hostsByGroup[hostInfo.Group] = append(hostsByGroup[hostInfo.Group], host)
	}
	bestIPsMu.RLock()
	responseGroups := make([]groupStatus, 0, len(groups))
	for _, group := range groups {
		hosts := hostsByGroup[group]
		sort.Strings(hosts)
		responseGroups = append(responseGroups, groupStatus{Name: group, Hosts: hosts, ManualIP: manualIPs[group], BestIP: bestIPsByGroup[group]})
	}
	bestIPsMu.RUnlock()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(manualSettingsResponse{ManualMode: manualMode, Groups: responseGroups})
}

const configPage = `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>优选域名 · 控制台</title><style>
:root{color-scheme:dark;--ink:#f5f7f4;--muted:#9ca6a1;--paper:#101512;--panel:#1a211c;--line:#303a32;--accent:#c5f451;--danger:#ff9b7e}*{box-sizing:border-box}body{margin:0;background:radial-gradient(circle at 85% 0,#263723 0,transparent 34rem),var(--paper);color:var(--ink);font-family:ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;min-height:100vh}main{max-width:900px;margin:auto;padding:68px 24px}header{display:flex;justify-content:space-between;gap:24px;align-items:start;border-bottom:1px solid var(--line);padding-bottom:38px}h1{font-size:clamp(2.2rem,7vw,4.8rem);letter-spacing:-.075em;line-height:.9;margin:0;text-transform:uppercase}.eyebrow{margin:0 0 14px;color:var(--accent);font-size:.75rem;letter-spacing:.16em;font-weight:700}.lede{color:var(--muted);max-width:32rem;line-height:1.6;margin:18px 0 0}.mode{display:flex;align-items:center;gap:12px;background:var(--panel);border:1px solid var(--line);border-radius:999px;padding:8px 12px 8px 16px;white-space:nowrap}.switch{appearance:none;width:46px;height:26px;border-radius:20px;background:#465048;position:relative;cursor:pointer;transition:.2s}.switch:checked{background:var(--accent)}.switch:before{content:"";position:absolute;top:4px;left:4px;width:18px;height:18px;border-radius:50%;background:white;transition:.2s}.switch:checked:before{transform:translateX(20px)}.groups{margin-top:34px;display:grid;gap:12px}.group{background:color-mix(in srgb,var(--panel) 90%,transparent);border:1px solid var(--line);border-radius:14px;padding:20px;display:grid;grid-template-columns:1fr minmax(190px,280px);gap:20px;align-items:center}.name{font-size:1.25rem;font-weight:750;letter-spacing:-.035em}.hosts{color:var(--muted);font-size:.85rem;line-height:1.5;margin-top:7px}.current{font-size:.8rem;color:var(--accent);margin-top:12px}.field{display:grid;gap:7px}.field label{font-size:.72rem;letter-spacing:.08em;color:var(--muted);text-transform:uppercase}.field input{width:100%;padding:12px 13px;border:1px solid var(--line);background:#0d120e;color:var(--ink);border-radius:8px;font:inherit;outline:none}.field input:focus{border-color:var(--accent);box-shadow:0 0 0 3px #c5f4511a}.field input:disabled{opacity:.4;cursor:not-allowed}footer{display:flex;align-items:center;justify-content:space-between;gap:18px;margin-top:26px}.note{color:var(--muted);font-size:.84rem;line-height:1.5}button{border:0;border-radius:8px;background:var(--accent);color:#17200e;padding:13px 18px;font:700 .9rem inherit;cursor:pointer}button:disabled{opacity:.55;cursor:wait}#message{min-height:1.5em;margin:18px 0 0;color:var(--muted)}#message.error{color:var(--danger)}@media(max-width:600px){main{padding:42px 18px}header,.group,footer{display:block}.mode{margin-top:26px;width:max-content}.group{padding:18px}.field{margin-top:18px}button{margin-top:18px;width:100%}}</style></head><body><main><header><div><p class="eyebrow">Cloudflare DNS / 37377</p><h1>优选控制台</h1><p class="lede">自动模式持续测量并更新最优节点。手动模式会暂停自动任务，保存后立即将每个分组的 IPv4 写入关联 DNS。</p></div><label class="mode"><span id="modeText">自动模式</span><input class="switch" id="mode" type="checkbox"></label></header><section class="groups" id="groups"></section><footer><p class="note">保存手动配置时，所有分组都必须填写有效 IPv4 地址。</p><button id="save">保存并更新 DNS</button></footer><p id="message" role="status"></p></main><script>const mode=document.querySelector('#mode'),modeText=document.querySelector('#modeText'),groups=document.querySelector('#groups'),save=document.querySelector('#save'),message=document.querySelector('#message');let data;function escapeHTML(v){const d=document.createElement('div');d.textContent=v;return d.innerHTML}function render(){mode.checked=data.manualMode;modeText.textContent=data.manualMode?'手动模式':'自动模式';groups.innerHTML=data.groups.map(g=>'<article class="group"><div><div class="name">'+escapeHTML(g.name)+'</div><div class="hosts">'+(g.hosts.length?g.hosts.map(escapeHTML).join('<br>'):'未关联域名')+'</div><div class="current">当前生效：'+escapeHTML(g.bestIP||'尚未选择')+'</div></div><div class="field"><label for="ip-'+escapeHTML(g.name)+'">手动 IPv4</label><input id="ip-'+escapeHTML(g.name)+'" data-group="'+escapeHTML(g.name)+'" value="'+escapeHTML(g.manualIP||'')+'" placeholder="例如 203.0.113.10" '+(data.manualMode?'':'disabled')+'></div></article>').join('')}function state(){modeText.textContent=mode.checked?'手动模式':'自动模式';document.querySelectorAll('[data-group]').forEach(i=>i.disabled=!mode.checked)}async function load(){const r=await fetch('/api/manual-settings');if(!r.ok)throw Error('无法读取配置');data=await r.json();render()}mode.addEventListener('change',state);save.addEventListener('click',async()=>{const manualIPs={};document.querySelectorAll('[data-group]').forEach(i=>manualIPs[i.dataset.group]=i.value.trim());save.disabled=true;message.className='';message.textContent='正在保存并更新 DNS…';try{const r=await fetch('/api/manual-settings',{method:'PUT',headers:{'Content-Type':'application/json'},body:JSON.stringify({manualMode:mode.checked,manualIPs})});if(!r.ok)throw Error(await r.text());data=await r.json();render();message.textContent=mode.checked?'已保存，DNS 更新请求已执行。':'已切换为自动模式。'}catch(e){message.className='error';message.textContent='保存失败：'+e.message}finally{save.disabled=false}});load().catch(e=>{message.className='error';message.textContent=e.message})</script></body></html>`
