export namespace pipeline {
	
	export class Job {
	    taskID: string;
	    title: string;
	    videoPath: string;
	    stage: string;
	    progress: number;
	    progressMsg?: string;
	    transcriptPath?: string;
	    srtPath?: string;
	    error?: string;
	    retries: number;
	    model?: string;
	    language?: string;
	    duration?: number;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.taskID = source["taskID"];
	        this.title = source["title"];
	        this.videoPath = source["videoPath"];
	        this.stage = source["stage"];
	        this.progress = source["progress"];
	        this.progressMsg = source["progressMsg"];
	        this.transcriptPath = source["transcriptPath"];
	        this.srtPath = source["srtPath"];
	        this.error = source["error"];
	        this.retries = source["retries"];
	        this.model = source["model"];
	        this.language = source["language"];
	        this.duration = source["duration"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class SavedTranscript {
	    language: string;
	    model: string;
	    segments: transcribe.Segment[];
	    fullText: string;
	    duration: number;
	    hits?: proofread.Hit[];
	
	    static createFrom(source: any = {}) {
	        return new SavedTranscript(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.model = source["model"];
	        this.segments = this.convertValues(source["segments"], transcribe.Segment);
	        this.fullText = source["fullText"];
	        this.duration = source["duration"];
	        this.hits = this.convertValues(source["hits"], proofread.Hit);
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

export namespace proofread {
	
	export class Entry {
	    id: string;
	    right: string;
	    wrong: string[];
	    category: string;
	    scope?: string;
	    source: string;
	    confidence?: number;
	    hitCount: number;
	    createdAt: string;
	    lastSeen?: string;
	    contextExample?: string;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.right = source["right"];
	        this.wrong = source["wrong"];
	        this.category = source["category"];
	        this.scope = source["scope"];
	        this.source = source["source"];
	        this.confidence = source["confidence"];
	        this.hitCount = source["hitCount"];
	        this.createdAt = source["createdAt"];
	        this.lastSeen = source["lastSeen"];
	        this.contextExample = source["contextExample"];
	    }
	}
	export class Hit {
	    segmentIndex: number;
	    start: number;
	    end: number;
	    entryID: string;
	    original: string;
	    replacement: string;
	
	    static createFrom(source: any = {}) {
	        return new Hit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.segmentIndex = source["segmentIndex"];
	        this.start = source["start"];
	        this.end = source["end"];
	        this.entryID = source["entryID"];
	        this.original = source["original"];
	        this.replacement = source["replacement"];
	    }
	}

}

export namespace scribe {
	
	export class ModelSummary {
	    key: string;
	    filename: string;
	    url: string;
	    bytes: number;
	    label: string;
	    installed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ModelSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.filename = source["filename"];
	        this.url = source["url"];
	        this.bytes = source["bytes"];
	        this.label = source["label"];
	        this.installed = source["installed"];
	    }
	}
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
	export class TranscribeSettings {
	    autoEnabled: boolean;
	    model: string;
	    language: string;
	
	    static createFrom(source: any = {}) {
	        return new TranscribeSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.autoEnabled = source["autoEnabled"];
	        this.model = source["model"];
	        this.language = source["language"];
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

export namespace transcribe {
	
	export class Segment {
	    start: number;
	    end: number;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new Segment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = source["start"];
	        this.end = source["end"];
	        this.text = source["text"];
	    }
	}

}

