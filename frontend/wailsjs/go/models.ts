export namespace main {
	
	export class EnvConfig {
	    name: string;
	    description: string;
	    variables: Record<string, string>;
	    provider: string;
	    templates?: Record<string, string>;
	    icon?: string;
	    attribution_header: string;
	    disable_nonessential_traffic: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.variables = source["variables"];
	        this.provider = source["provider"];
	        this.templates = source["templates"];
	        this.icon = source["icon"];
	        this.attribution_header = source["attribution_header"];
	        this.disable_nonessential_traffic = source["disable_nonessential_traffic"];
	    }
	}
	export class Config {
	    current_env: string;
	    current_env_claude: string;
	    current_env_codex: string;
	    current_env_gemini: string;
	    environments: EnvConfig[];
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current_env = source["current_env"];
	        this.current_env_claude = source["current_env_claude"];
	        this.current_env_codex = source["current_env_codex"];
	        this.current_env_gemini = source["current_env_gemini"];
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
	    website?: string;
	    tips?: string;
	    enable_platform: string[];
	    enabled_in_claude: boolean;
	    enabled_in_codex: boolean;
	    enabled_in_gemini: boolean;
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
	        this.website = source["website"];
	        this.tips = source["tips"];
	        this.enable_platform = source["enable_platform"];
	        this.enabled_in_claude = source["enabled_in_claude"];
	        this.enabled_in_codex = source["enabled_in_codex"];
	        this.enabled_in_gemini = source["enabled_in_gemini"];
	        this.missing_placeholders = source["missing_placeholders"];
	    }
	}
	export class MCPTestResult {
	    success: boolean;
	    message: string;
	    latency: number;
	
	    static createFrom(source: any = {}) {
	        return new MCPTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.latency = source["latency"];
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
	export class Skill {
	    name: string;
	    content: string;
	    enable_platform: string[];
	    enabled_in_claude: boolean;
	    enabled_in_codex: boolean;
	    enabled_in_gemini: boolean;
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
	        this.name = source["name"];
	        this.content = source["content"];
	        this.enable_platform = source["enable_platform"];
	        this.enabled_in_claude = source["enabled_in_claude"];
	        this.enabled_in_codex = source["enabled_in_codex"];
	        this.enabled_in_gemini = source["enabled_in_gemini"];
	        this.frontmatter_name = source["frontmatter_name"];
	        this.description = source["description"];
	        this.has_frontmatter = source["has_frontmatter"];
	        this.has_name = source["has_name"];
	        this.has_description = source["has_description"];
	        this.frontmatter_error = source["frontmatter_error"];
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
	
	    static createFrom(source: any = {}) {
	        return new UptimeSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.interval_seconds = source["interval_seconds"];
	        this.timeout_seconds = source["timeout_seconds"];
	        this.keep_last = source["keep_last"];
	    }
	}
	export class UptimeSnapshot {
	    settings: UptimeSettings;
	    groups: RotationGroup[];
	    history: Record<string, Array<UptimeCheck>>;
	    urls: Record<string, string>;
	    now: number;
	
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

}

