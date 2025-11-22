export namespace main {
	
	export class EnvConfig {
	    name: string;
	    description: string;
	    variables: Record<string, string>;
	    provider: string;
	    templates?: Record<string, string>;
	
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

}

