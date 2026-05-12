export namespace scribe {
	
	export class ProxyStatus {
	    running: boolean;
	    interceptorAddr: string;
	    apiAddr: string;
	    lastError?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.interceptorAddr = source["interceptorAddr"];
	        this.apiAddr = source["apiAddr"];
	        this.lastError = source["lastError"];
	    }
	}
	export class VersionInfo {
	    app: string;
	    core: string;
	    commit: string;
	    buildDate: string;
	
	    static createFrom(source: any = {}) {
	        return new VersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.app = source["app"];
	        this.core = source["core"];
	        this.commit = source["commit"];
	        this.buildDate = source["buildDate"];
	    }
	}

}

export namespace sphkit {
	
	export class Config {
	    downloadDir: string;
	    interceptorAddr: string;
	    apiAddr: string;
	    maxRunning: number;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.downloadDir = source["downloadDir"];
	        this.interceptorAddr = source["interceptorAddr"];
	        this.apiAddr = source["apiAddr"];
	        this.maxRunning = source["maxRunning"];
	    }
	}
	export class TaskSummary {
	    id: string;
	    title: string;
	    spec: string;
	    size: number;
	    downloaded: number;
	    speed: number;
	    status: string;
	    path: string;
	    filename: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.spec = source["spec"];
	        this.size = source["size"];
	        this.downloaded = source["downloaded"];
	        this.speed = source["speed"];
	        this.status = source["status"];
	        this.path = source["path"];
	        this.filename = source["filename"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class TaskListResult {
	    tasks: TaskSummary[];
	    total: number;
	    page: number;
	    pageSize: number;
	
	    static createFrom(source: any = {}) {
	        return new TaskListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tasks = this.convertValues(source["tasks"], TaskSummary);
	        this.total = source["total"];
	        this.page = source["page"];
	        this.pageSize = source["pageSize"];
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

