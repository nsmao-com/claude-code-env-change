export namespace main {
	
	export class RouteUpstream {
	    base_url: string;
	    api_key?: string;
	    weight?: number;
	
	    static createFrom(source: any = {}) {
	        return new RouteUpstream(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.base_url = source["base_url"];
	        this.api_key = source["api_key"];
	        this.weight = source["weight"];
	    }
	}
	export class APIRoute {
	    name: string;
	    description?: string;
	    source_format: string;
	    target_format: string;
	    base_url: string;
	    api_key?: string;
	    model_mapping?: Record<string, string>;
	    default_model?: string;
	    enabled: boolean;
	    fallbacks?: RouteUpstream[];
	    failure_threshold?: number;
	    cooldown_seconds?: number;
	    strategy?: string;
	    weight?: number;
	
	    static createFrom(source: any = {}) {
	        return new APIRoute(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.source_format = source["source_format"];
	        this.target_format = source["target_format"];
	        this.base_url = source["base_url"];
	        this.api_key = source["api_key"];
	        this.model_mapping = source["model_mapping"];
	        this.default_model = source["default_model"];
	        this.enabled = source["enabled"];
	        this.fallbacks = this.convertValues(source["fallbacks"], RouteUpstream);
	        this.failure_threshold = source["failure_threshold"];
	        this.cooldown_seconds = source["cooldown_seconds"];
	        this.strategy = source["strategy"];
	        this.weight = source["weight"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class BalanceResult {
	    amount: number;
	    currency: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new BalanceResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.amount = source["amount"];
	        this.currency = source["currency"];
	        this.message = source["message"];
	    }
	}
	export class BalanceSource {
	    provider: string;
	    environment: string;
	    adapter: string;

	    static createFrom(source: any = {}) {
	        return new BalanceSource(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.environment = source["environment"];
	        this.adapter = source["adapter"];
	    }
	}
	export class CliToolStatus {
	    id: string;
	    name: string;
	    command: string;
	    installed: boolean;
	    runnable: boolean;
	    current_version: string;
	    latest_version: string;
	    install_path: string;
	    install_method: string;
	    config_dir: string;
	    config_exists: boolean;
	    platform: string;
	    upgradable: boolean;
	    can_install: boolean;
	    error: string;
	    extra_paths: string[];
	    npm_package: string;
	
	    static createFrom(source: any = {}) {
	        return new CliToolStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.command = source["command"];
	        this.installed = source["installed"];
	        this.runnable = source["runnable"];
	        this.current_version = source["current_version"];
	        this.latest_version = source["latest_version"];
	        this.install_path = source["install_path"];
	        this.install_method = source["install_method"];
	        this.config_dir = source["config_dir"];
	        this.config_exists = source["config_exists"];
	        this.platform = source["platform"];
	        this.upgradable = source["upgradable"];
	        this.can_install = source["can_install"];
	        this.error = source["error"];
	        this.extra_paths = source["extra_paths"];
	        this.npm_package = source["npm_package"];
	    }
	}
	export class CliUpgradeResult {
	    id: string;
	    success: boolean;
	    message: string;
	    log: string;
	
	    static createFrom(source: any = {}) {
	        return new CliUpgradeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.success = source["success"];
	        this.message = source["message"];
	        this.log = source["log"];
	    }
	}
	export class CloudConfig {
	    enabled: boolean;
	    provider: string;
	    endpoint: string;
	    region: string;
	    bucket: string;
	    object_key: string;
	    access_key: string;
	    secret_key: string;
	    path_style: boolean;
	    passphrase?: string;
	    clear_secrets?: boolean;
	    auto_push: boolean;
	    auto_pull_on_start: boolean;
	    last_push_at?: number;
	    last_pull_at?: number;
	    last_error?: string;
	    remote_etag?: string;
	    local_baseline?: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.provider = source["provider"];
	        this.endpoint = source["endpoint"];
	        this.region = source["region"];
	        this.bucket = source["bucket"];
	        this.object_key = source["object_key"];
	        this.access_key = source["access_key"];
	        this.secret_key = source["secret_key"];
	        this.path_style = source["path_style"];
	        this.passphrase = source["passphrase"];
	        this.clear_secrets = source["clear_secrets"];
	        this.auto_push = source["auto_push"];
	        this.auto_pull_on_start = source["auto_pull_on_start"];
	        this.last_push_at = source["last_push_at"];
	        this.last_pull_at = source["last_pull_at"];
	        this.last_error = source["last_error"];
	        this.remote_etag = source["remote_etag"];
	        this.local_baseline = source["local_baseline"];
	    }
	}
	export class CloudSyncResult {
	    success: boolean;
	    message: string;
	    latency: number;
	
	    static createFrom(source: any = {}) {
	        return new CloudSyncResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.latency = source["latency"];
	    }
	}
	export class CloudSyncStatus {
	    enabled: boolean;
	    configured: boolean;
	    pushing: boolean;
	    last_push_at?: number;
	    last_pull_at?: number;
	    last_error?: string;
	    object_key?: string;
	    provider?: string;
	
	    static createFrom(source: any = {}) {
	        return new CloudSyncStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.configured = source["configured"];
	        this.pushing = source["pushing"];
	        this.last_push_at = source["last_push_at"];
	        this.last_pull_at = source["last_pull_at"];
	        this.last_error = source["last_error"];
	        this.object_key = source["object_key"];
	        this.provider = source["provider"];
	    }
	}
	export class EnvConfig {
	    universal_id?: string;
	    name: string;
	    description: string;
	    variables: Record<string, string>;
	    provider: string;
	    templates?: Record<string, string>;
	    icon?: string;
	    upstream_format?: string;
	    official_login?: boolean;
	    attribution_header: string;
	    disable_nonessential_traffic: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.universal_id = source["universal_id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.variables = source["variables"];
	        this.provider = source["provider"];
	        this.templates = source["templates"];
	        this.icon = source["icon"];
	        this.upstream_format = source["upstream_format"];
	        this.official_login = source["official_login"];
	        this.attribution_header = source["attribution_header"];
	        this.disable_nonessential_traffic = source["disable_nonessential_traffic"];
	    }
	}
	export class Config {
	    current_env: string;
	    current_env_claude: string;
	    current_env_claude_desktop: string;
	    current_env_codex: string;
	    current_env_antigravity: string;
	    current_env_opencode: string;
	    current_envs_opencode: string[];
	    current_env_grok: string;
	    environments: EnvConfig[];
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current_env = source["current_env"];
	        this.current_env_claude = source["current_env_claude"];
	        this.current_env_claude_desktop = source["current_env_claude_desktop"];
	        this.current_env_codex = source["current_env_codex"];
	        this.current_env_antigravity = source["current_env_antigravity"];
	        this.current_env_opencode = source["current_env_opencode"];
	        this.current_envs_opencode = source["current_envs_opencode"];
	        this.current_env_grok = source["current_env_grok"];
	        this.environments = this.convertValues(source["environments"], EnvConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ConfigDirFile {
	    name: string;
	    path: string;
	    exists: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConfigDirFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.exists = source["exists"];
	    }
	}
	export class ConfigDirInfo {
	    id: string;
	    name: string;
	    dir: string;
	    exists: boolean;
	    files: ConfigDirFile[];
	
	    static createFrom(source: any = {}) {
	        return new ConfigDirInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.dir = source["dir"];
	        this.exists = source["exists"];
	        this.files = this.convertValues(source["files"], ConfigDirFile);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class CostOverview {
	    today: number;
	    month: number;
	    requests: number;
	    unpriced: number;
	    input: number;
	    output: number;
	    daily_exceeded: boolean;
	    monthly_exceeded: boolean;
	    by_route: Record<string, number>;

	    static createFrom(source: any = {}) {
	        return new CostOverview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.today = source["today"];
	        this.month = source["month"];
	        this.requests = source["requests"];
	        this.unpriced = source["unpriced"];
	        this.input = source["input"];
	        this.output = source["output"];
	        this.daily_exceeded = source["daily_exceeded"];
	        this.monthly_exceeded = source["monthly_exceeded"];
	        this.by_route = source["by_route"];
	    }
	}
	export class CostSettings {
	    daily_budget: number;
	    monthly_budget: number;
	    multipliers: Record<string, number>;
	    balance_sources: BalanceSource[];

	    static createFrom(source: any = {}) {
	        return new CostSettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.daily_budget = source["daily_budget"];
	        this.monthly_budget = source["monthly_budget"];
	        this.multipliers = source["multipliers"];
	        this.balance_sources = this.convertValues(source["balance_sources"], BalanceSource);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class DiagnosticItem {
	    name: string;
	    status: string;
	    message: string;
	    action: string;

	    static createFrom(source: any = {}) {
	        return new DiagnosticItem(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.action = source["action"];
	    }
	}
	export class DiscoveredModel {
	    id: string;
	    name: string;

	    static createFrom(source: any = {}) {
	        return new DiscoveredModel(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	
	export class EnvUsageSummary {
	    provider: string;
	    requests: number;
	    input_tokens: number;
	    output_tokens: number;
	    cache_read_tokens: number;
	    cache_write_tokens: number;
	    total_cost: number;
	    last_timestamp?: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvUsageSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.requests = source["requests"];
	        this.input_tokens = source["input_tokens"];
	        this.output_tokens = source["output_tokens"];
	        this.cache_read_tokens = source["cache_read_tokens"];
	        this.cache_write_tokens = source["cache_write_tokens"];
	        this.total_cost = source["total_cost"];
	        this.last_timestamp = source["last_timestamp"];
	    }
	}
	export class FileChange {
	    path: string;
	    before: string;
	    after: string;
	    changed: boolean;

	    static createFrom(source: any = {}) {
	        return new FileChange(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.before = source["before"];
	        this.after = source["after"];
	        this.changed = source["changed"];
	    }
	}
	export class RouterLogEntry {
	    time: string;
	    route: string;
	    path: string;
	    model?: string;
	    status_code: number;
	    duration_ms: number;
	    error?: string;
	    upstream?: string;
	    failover?: string;
	    input_tokens?: number;
	    output_tokens?: number;
	    cache_read_tokens?: number;
	    cache_write_tokens?: number;
	    first_token_ms?: number;
	    usage_reported?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RouterLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.route = source["route"];
	        this.path = source["path"];
	        this.model = source["model"];
	        this.status_code = source["status_code"];
	        this.duration_ms = source["duration_ms"];
	        this.error = source["error"];
	        this.upstream = source["upstream"];
	        this.failover = source["failover"];
	        this.input_tokens = source["input_tokens"];
	        this.output_tokens = source["output_tokens"];
	        this.cache_read_tokens = source["cache_read_tokens"];
	        this.cache_write_tokens = source["cache_write_tokens"];
	        this.first_token_ms = source["first_token_ms"];
	        this.usage_reported = source["usage_reported"];
	    }
	}
	export class RouteStats {
	    total_requests: number;
	    failed_requests: number;
	    failover_count: number;
	    last_error?: string;
	    last_request_at?: number;
	
	    static createFrom(source: any = {}) {
	        return new RouteStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_requests = source["total_requests"];
	        this.failed_requests = source["failed_requests"];
	        this.failover_count = source["failover_count"];
	        this.last_error = source["last_error"];
	        this.last_request_at = source["last_request_at"];
	    }
	}
	export class GatewayStatus {
	    running: boolean;
	    port: number;
	    stats: Record<string, RouteStats>;
	    logs: RouterLogEntry[];
	
	    static createFrom(source: any = {}) {
	        return new GatewayStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.port = source["port"];
	        this.stats = this.convertValues(source["stats"], RouteStats, true);
	        this.logs = this.convertValues(source["logs"], RouterLogEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HeatmapData {
	    date: string;
	    requests: number;
	    tokens: number;
	    cost: number;
	
	    static createFrom(source: any = {}) {
	        return new HeatmapData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.requests = source["requests"];
	        this.tokens = source["tokens"];
	        this.cost = source["cost"];
	    }
	}
	export class HistoryPreview {
	    token: string;
	    changes: FileChange[];

	    static createFrom(source: any = {}) {
	        return new HistoryPreview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.changes = this.convertValues(source["changes"], FileChange);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class HistorySummary {
	    id: string;
	    at: number;
	    reason: string;
	    paths: string[];

	    static createFrom(source: any = {}) {
	        return new HistorySummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.at = source["at"];
	        this.reason = source["reason"];
	        this.paths = source["paths"];
	    }
	}
	export class HourlyStat {
	    hour: string;
	    requests: number;
	    input_tokens: number;
	    output_tokens: number;
	    cost: number;
	
	    static createFrom(source: any = {}) {
	        return new HourlyStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hour = source["hour"];
	        this.requests = source["requests"];
	        this.input_tokens = source["input_tokens"];
	        this.output_tokens = source["output_tokens"];
	        this.cost = source["cost"];
	    }
	}
	export class MCPServer {
	    name: string;
	    type: string;
	    command?: string;
	    args?: string[];
	    env?: Record<string, string>;
	    url?: string;
	    headers?: Record<string, string>;
	    website?: string;
	    tips?: string;
	    enable_platform: string[];
	    enabled_in_claude: boolean;
	    enabled_in_codex: boolean;
	    enabled_in_antigravity: boolean;
	    enabled_in_opencode: boolean;
	    enabled_in_grok: boolean;
	    enabled_in_claude_desktop: boolean;
	    missing_placeholders: string[];
	
	    static createFrom(source: any = {}) {
	        return new MCPServer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.env = source["env"];
	        this.url = source["url"];
	        this.headers = source["headers"];
	        this.website = source["website"];
	        this.tips = source["tips"];
	        this.enable_platform = source["enable_platform"];
	        this.enabled_in_claude = source["enabled_in_claude"];
	        this.enabled_in_codex = source["enabled_in_codex"];
	        this.enabled_in_antigravity = source["enabled_in_antigravity"];
	        this.enabled_in_opencode = source["enabled_in_opencode"];
	        this.enabled_in_grok = source["enabled_in_grok"];
	        this.enabled_in_claude_desktop = source["enabled_in_claude_desktop"];
	        this.missing_placeholders = source["missing_placeholders"];
	    }
	}
	export class MCPTestResult {
	    success: boolean;
	    message: string;
	    latency: number;
	    stage?: string;
	    tools?: string[];
	    protocol?: string;
	
	    static createFrom(source: any = {}) {
	        return new MCPTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.latency = source["latency"];
	        this.stage = source["stage"];
	        this.tools = source["tools"];
	        this.protocol = source["protocol"];
	    }
	}
	export class McpMarketItem {
	    id: string;
	    name: string;
	    title: string;
	    description: string;
	    website: string;
	    version: string;
	    type: string;
	    command: string;
	    args: string[];
	    url: string;
	    hint: string;
	
	    static createFrom(source: any = {}) {
	        return new McpMarketItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.website = source["website"];
	        this.version = source["version"];
	        this.type = source["type"];
	        this.command = source["command"];
	        this.args = source["args"];
	        this.url = source["url"];
	        this.hint = source["hint"];
	    }
	}
	export class McpMarketPage {
	    items: McpMarketItem[];
	    next: string;
	    warning: string;
	
	    static createFrom(source: any = {}) {
	        return new McpMarketPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], McpMarketItem);
	        this.next = source["next"];
	        this.warning = source["warning"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ModelProfile {
	    provider: string;
	    environment: string;
	    model: string;
	    context: number;
	    output: number;
	    compact: number;
	    input_price: number;
	    output_price: number;
	    cache_read_price: number;
	    cache_write_price: number;

	    static createFrom(source: any = {}) {
	        return new ModelProfile(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.environment = source["environment"];
	        this.model = source["model"];
	        this.context = source["context"];
	        this.output = source["output"];
	        this.compact = source["compact"];
	        this.input_price = source["input_price"];
	        this.output_price = source["output_price"];
	        this.cache_read_price = source["cache_read_price"];
	        this.cache_write_price = source["cache_write_price"];
	    }
	}
	export class ModelStats {
	    requests: number;
	    tokens: number;
	    cost: number;
	
	    static createFrom(source: any = {}) {
	        return new ModelStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.requests = source["requests"];
	        this.tokens = source["tokens"];
	        this.cost = source["cost"];
	    }
	}
	export class OutboundProxySettings {
	    enabled: boolean;
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new OutboundProxySettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.url = source["url"];
	    }
	}
	export class ProjectPreset {
	    name: string;
	    directory: string;
	    provider: string;
	    environment: string;
	    model: string;
	    mcp: string[];
	    skills: string[];
	    prompt: string;

	    static createFrom(source: any = {}) {
	        return new ProjectPreset(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.directory = source["directory"];
	        this.provider = source["provider"];
	        this.environment = source["environment"];
	        this.model = source["model"];
	        this.mcp = source["mcp"];
	        this.skills = source["skills"];
	        this.prompt = source["prompt"];
	    }
	}
	export class PromptFile {
	    provider: string;
	    path: string;
	    content: string;
	    exists: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PromptFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.path = source["path"];
	        this.content = source["content"];
	        this.exists = source["exists"];
	    }
	}
	export class PromptPreset {
	    name: string;
	    content: string;

	    static createFrom(source: any = {}) {
	        return new PromptPreset(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.content = source["content"];
	    }
	}
	export class ProviderPreset {
	    id: string;
	    name: string;
	    provider: string;
	    icon: string;
	    description: string;
	    variables: Record<string, string>;
	    model_choices?: string[];
	    website?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProviderPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.provider = source["provider"];
	        this.icon = source["icon"];
	        this.description = source["description"];
	        this.variables = source["variables"];
	        this.model_choices = source["model_choices"];
	        this.website = source["website"];
	    }
	}
	export class ProxyTestResult {
	    success: boolean;
	    message: string;
	    latency: number;
	
	    static createFrom(source: any = {}) {
	        return new ProxyTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.latency = source["latency"];
	    }
	}
	export class RotationGroup {
	    name: string;
	    provider: string;
	    env_names: string[];
	    enabled: boolean;
	    failure_threshold: number;
	
	    static createFrom(source: any = {}) {
	        return new RotationGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.provider = source["provider"];
	        this.env_names = source["env_names"];
	        this.enabled = source["enabled"];
	        this.failure_threshold = source["failure_threshold"];
	    }
	}
	

	export class RouterConfig {
	    port: number;
	    auto_start: boolean;
	    routes: APIRoute[];
	    app_routing?: Record<string, boolean>;
	
	    static createFrom(source: any = {}) {
	        return new RouterConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.auto_start = source["auto_start"];
	        this.routes = this.convertValues(source["routes"], APIRoute);
	        this.app_routing = source["app_routing"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class RouterLogPage {
	    items: RouterLogEntry[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new RouterLogPage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], RouterLogEntry);
	        this.total = source["total"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class RouterLogQuery {
	    route: string;
	    keyword: string;
	    only_errors: boolean;
	    limit: number;
	    offset: number;
	
	    static createFrom(source: any = {}) {
	        return new RouterLogQuery(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.route = source["route"];
	        this.keyword = source["keyword"];
	        this.only_errors = source["only_errors"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	    }
	}
	export class RouterTestResult {
	    success: boolean;
	    message: string;
	    latency: number;
	
	    static createFrom(source: any = {}) {
	        return new RouterTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.latency = source["latency"];
	    }
	}
	export class SessionMessage {
	    role: string;
	    text: string;
	    at: string;

	    static createFrom(source: any = {}) {
	        return new SessionMessage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.text = source["text"];
	        this.at = source["at"];
	    }
	}
	export class SessionSummary {
	    id: string;
	    session_id: string;
	    provider: string;
	    title: string;
	    project: string;
	    model: string;
	    updated: number;
	    messages: number;
	    archived: boolean;
	    path: string;
	    warning?: string;

	    static createFrom(source: any = {}) {
	        return new SessionSummary(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.session_id = source["session_id"];
	        this.provider = source["provider"];
	        this.title = source["title"];
	        this.project = source["project"];
	        this.model = source["model"];
	        this.updated = source["updated"];
	        this.messages = source["messages"];
	        this.archived = source["archived"];
	        this.path = source["path"];
	        this.warning = source["warning"];
	    }
	}
	export class SessionDetail {
	    session: SessionSummary;
	    messages: SessionMessage[];
	    total: number;

	    static createFrom(source: any = {}) {
	        return new SessionDetail(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session = this.convertValues(source["session"], SessionSummary);
	        this.messages = this.convertValues(source["messages"], SessionMessage);
	        this.total = source["total"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

	export class SessionPage {
	    items: SessionSummary[];
	    total: number;
	    warnings: string[];

	    static createFrom(source: any = {}) {
	        return new SessionPage(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.items = this.convertValues(source["items"], SessionSummary);
	        this.total = source["total"];
	        this.warnings = source["warnings"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SessionQuery {
	    keyword: string;
	    provider: string;
	    project: string;
	    archived: boolean;
	    offset: number;
	    limit: number;
	    from: number;
	    to: number;

	    static createFrom(source: any = {}) {
	        return new SessionQuery(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.keyword = source["keyword"];
	        this.provider = source["provider"];
	        this.project = source["project"];
	        this.archived = source["archived"];
	        this.offset = source["offset"];
	        this.limit = source["limit"];
	        this.from = source["from"];
	        this.to = source["to"];
	    }
	}

	export class SkillSource {
	    repo: string;
	    path: string;
	    ref: string;
	    revision: string;
	    hashes: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new SkillSource(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.repo = source["repo"];
	        this.path = source["path"];
	        this.ref = source["ref"];
	        this.revision = source["revision"];
	        this.hashes = source["hashes"];
	    }
	}
	export class Skill {
	    executable?: Record<string, boolean>;
	    files?: Record<string, Array<number>>;
	    source?: SkillSource;
	    name: string;
	    content: string;
	    enable_platform: string[];
	    enabled_in_claude: boolean;
	    enabled_in_codex: boolean;
	    enabled_in_antigravity: boolean;
	    enabled_in_opencode: boolean;
	    enabled_in_grok: boolean;
	    frontmatter_name: string;
	    description: string;
	    has_frontmatter: boolean;
	    has_name: boolean;
	    has_description: boolean;
	    frontmatter_error: string;
	
	    static createFrom(source: any = {}) {
	        return new Skill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.executable = source["executable"];
	        this.files = source["files"];
	        this.source = this.convertValues(source["source"], SkillSource);
	        this.name = source["name"];
	        this.content = source["content"];
	        this.enable_platform = source["enable_platform"];
	        this.enabled_in_claude = source["enabled_in_claude"];
	        this.enabled_in_codex = source["enabled_in_codex"];
	        this.enabled_in_antigravity = source["enabled_in_antigravity"];
	        this.enabled_in_opencode = source["enabled_in_opencode"];
	        this.enabled_in_grok = source["enabled_in_grok"];
	        this.frontmatter_name = source["frontmatter_name"];
	        this.description = source["description"];
	        this.has_frontmatter = source["has_frontmatter"];
	        this.has_name = source["has_name"];
	        this.has_description = source["has_description"];
	        this.frontmatter_error = source["frontmatter_error"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SkillMarketItem {
	    id: string;
	    name: string;
	    description: string;
	    source: string;
	    repo: string;
	    path: string;
	    builtin: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SkillMarketItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.source = source["source"];
	        this.repo = source["repo"];
	        this.path = source["path"];
	        this.builtin = source["builtin"];
	    }
	}
	export class SkillPreset {
	    name: string;
	    description: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new SkillPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.content = source["content"];
	    }
	}

	export class SkillUpdate {
	    name: string;
	    token: string;
	    revision: string;
	    changes: string[];
	    conflicts: string[];
	    platform_conflicts: string[];
	    details: FileChange[];

	    static createFrom(source: any = {}) {
	        return new SkillUpdate(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.token = source["token"];
	        this.revision = source["revision"];
	        this.changes = source["changes"];
	        this.conflicts = source["conflicts"];
	        this.platform_conflicts = source["platform_conflicts"];
	        this.details = this.convertValues(source["details"], FileChange);
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UsageStats {
	    total_requests: number;
	    total_input_tokens: number;
	    total_output_tokens: number;
	    total_cache_read: number;
	    total_cache_write: number;
	    total_cost: number;
	    by_model: Record<string, ModelStats>;
	    series: HourlyStat[];
	
	    static createFrom(source: any = {}) {
	        return new UsageStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total_requests = source["total_requests"];
	        this.total_input_tokens = source["total_input_tokens"];
	        this.total_output_tokens = source["total_output_tokens"];
	        this.total_cache_read = source["total_cache_read"];
	        this.total_cache_write = source["total_cache_write"];
	        this.total_cost = source["total_cost"];
	        this.by_model = this.convertValues(source["by_model"], ModelStats, true);
	        this.series = this.convertValues(source["series"], HourlyStat);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class StatsOverview {
	    stats: UsageStats;
	    heatmap: HeatmapData[];
	    log_directory: string;
	    env_summary: Record<string, EnvUsageSummary>;
	
	    static createFrom(source: any = {}) {
	        return new StatsOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stats = this.convertValues(source["stats"], UsageStats);
	        this.heatmap = this.convertValues(source["heatmap"], HeatmapData);
	        this.log_directory = source["log_directory"];
	        this.env_summary = this.convertValues(source["env_summary"], EnvUsageSummary, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UniversalProvider {
	    name: string;
	    base_url: string;
	    api_key: string;
	    model: string;
	    format: string;
	    providers: string[];

	    static createFrom(source: any = {}) {
	        return new UniversalProvider(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.base_url = source["base_url"];
	        this.api_key = source["api_key"];
	        this.model = source["model"];
	        this.format = source["format"];
	        this.providers = source["providers"];
	    }
	}
	export class UpdateInfo {
	    available: boolean;
	    current_version: string;
	    latest_version: string;
	    release_name: string;
	    release_notes: string;
	    published_at: string;
	    download_url: string;
	    asset_name: string;
	    asset_size: number;
	    asset_digest: string;
	    release_url: string;
	    can_apply: boolean;
	    is_dev: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.current_version = source["current_version"];
	        this.latest_version = source["latest_version"];
	        this.release_name = source["release_name"];
	        this.release_notes = source["release_notes"];
	        this.published_at = source["published_at"];
	        this.download_url = source["download_url"];
	        this.asset_name = source["asset_name"];
	        this.asset_size = source["asset_size"];
	        this.asset_digest = source["asset_digest"];
	        this.release_url = source["release_url"];
	        this.can_apply = source["can_apply"];
	        this.is_dev = source["is_dev"];
	        this.message = source["message"];
	    }
	}
	export class UpstreamMetric {
	    route: string;
	    host: string;
	    requests: number;
	    failures: number;
	    consecutive: number;
	    latency: number;
	    cool_until: number;
	    probing: boolean;

	    static createFrom(source: any = {}) {
	        return new UpstreamMetric(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.route = source["route"];
	        this.host = source["host"];
	        this.requests = source["requests"];
	        this.failures = source["failures"];
	        this.consecutive = source["consecutive"];
	        this.latency = source["latency"];
	        this.cool_until = source["cool_until"];
	        this.probing = source["probing"];
	    }
	}
	export class UptimeCheck {
	    at: number;
	    success: boolean;
	    status_code: number;
	    latency_ms: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new UptimeCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = source["at"];
	        this.success = source["success"];
	        this.status_code = source["status_code"];
	        this.latency_ms = source["latency_ms"];
	        this.error = source["error"];
	    }
	}
	export class UptimeSettings {
	    enabled: boolean;
	    interval_seconds: number;
	    timeout_seconds: number;
	    keep_last: number;
	    probe_mode: string;
	
	    static createFrom(source: any = {}) {
	        return new UptimeSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.interval_seconds = source["interval_seconds"];
	        this.timeout_seconds = source["timeout_seconds"];
	        this.keep_last = source["keep_last"];
	        this.probe_mode = source["probe_mode"];
	    }
	}
	export class UptimeSnapshot {
	    settings: UptimeSettings;
	    groups: RotationGroup[];
	    history: Record<string, Array<UptimeCheck>>;
	    urls: Record<string, string>;
	    now: number;
	    last_rotation?: string;
	    last_rotation_error?: string;
	    last_rotation_at?: number;
	
	    static createFrom(source: any = {}) {
	        return new UptimeSnapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.settings = this.convertValues(source["settings"], UptimeSettings);
	        this.groups = this.convertValues(source["groups"], RotationGroup);
	        this.history = this.convertValues(source["history"], Array<UptimeCheck>, true);
	        this.urls = source["urls"];
	        this.now = source["now"];
	        this.last_rotation = source["last_rotation"];
	        this.last_rotation_error = source["last_rotation_error"];
	        this.last_rotation_at = source["last_rotation_at"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UsageRecord {
	    timestamp: string;
	    model: string;
	    input_tokens: number;
	    output_tokens: number;
	    cache_read_tokens: number;
	    cache_write_tokens: number;
	    total_cost: number;
	    session_id: string;
	    project_path: string;
	    provider?: string;
	
	    static createFrom(source: any = {}) {
	        return new UsageRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.model = source["model"];
	        this.input_tokens = source["input_tokens"];
	        this.output_tokens = source["output_tokens"];
	        this.cache_read_tokens = source["cache_read_tokens"];
	        this.cache_write_tokens = source["cache_write_tokens"];
	        this.total_cost = source["total_cost"];
	        this.session_id = source["session_id"];
	        this.project_path = source["project_path"];
	        this.provider = source["provider"];
	    }
	}

	export class WorkbenchConfig {
	    models: ModelProfile[];
	    prompts: PromptPreset[];
	    projects: ProjectPreset[];
	    costs: CostSettings;
	    archived_sessions: string[];

	    static createFrom(source: any = {}) {
	        return new WorkbenchConfig(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.models = this.convertValues(source["models"], ModelProfile);
	        this.prompts = this.convertValues(source["prompts"], PromptPreset);
	        this.projects = this.convertValues(source["projects"], ProjectPreset);
	        this.costs = this.convertValues(source["costs"], CostSettings);
	        this.archived_sessions = source["archived_sessions"];
	    }

		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

