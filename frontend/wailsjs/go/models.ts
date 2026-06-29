export namespace external {

	export class AddRequest {
	    url: string;
	    format?: string;
	    formatHint?: string;
	    cookieFile?: string;
	    subLangs?: string[];
	    title?: string;
	    site?: string;
	    duration?: number;

	    static createFrom(source: any = {}) {
	        return new AddRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.format = source["format"];
	        this.formatHint = source["formatHint"];
	        this.cookieFile = source["cookieFile"];
	        this.subLangs = source["subLangs"];
	        this.title = source["title"];
	        this.site = source["site"];
	        this.duration = source["duration"];
	    }
	}
	export class Format {
	    id: string;
	    label: string;
	    height: number;
	    fileSize: number;
	    ext: string;

	    static createFrom(source: any = {}) {
	        return new Format(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.height = source["height"];
	        this.fileSize = source["fileSize"];
	        this.ext = source["ext"];
	    }
	}
	export class ProbeResult {
	    url: string;
	    title: string;
	    site: string;
	    duration: number;
	    thumbnail?: string;
	    uploader?: string;
	    formats: Format[];
	    subLangs?: string[];

	    static createFrom(source: any = {}) {
	        return new ProbeResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.title = source["title"];
	        this.site = source["site"];
	        this.duration = source["duration"];
	        this.thumbnail = source["thumbnail"];
	        this.uploader = source["uploader"];
	        this.formats = this.convertValues(source["formats"], Format);
	        this.subLangs = source["subLangs"];
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
	export class Task {
	    id: string;
	    url: string;
	    title: string;
	    site: string;
	    duration: number;
	    thumbnail?: string;
	    format: string;
	    formatHint?: string;
	    cookieFile?: string;
	    subLangs?: string[];
	    status: string;
	    progress: number;
	    progressMsg?: string;
	    downloaded: number;
	    totalBytes: number;
	    speed: number;
	    eta: number;
	    path: string;
	    filename: string;
	    error?: string;
	    createdAt: string;
	    updatedAt: string;

	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.url = source["url"];
	        this.title = source["title"];
	        this.site = source["site"];
	        this.duration = source["duration"];
	        this.thumbnail = source["thumbnail"];
	        this.format = source["format"];
	        this.formatHint = source["formatHint"];
	        this.cookieFile = source["cookieFile"];
	        this.subLangs = source["subLangs"];
	        this.status = source["status"];
	        this.progress = source["progress"];
	        this.progressMsg = source["progressMsg"];
	        this.downloaded = source["downloaded"];
	        this.totalBytes = source["totalBytes"];
	        this.speed = source["speed"];
	        this.eta = source["eta"];
	        this.path = source["path"];
	        this.filename = source["filename"];
	        this.error = source["error"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

export namespace logbus {

	export class Entry {
	    timestamp: string;
	    level: string;
	    source: string;
	    message: string;

	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.level = source["level"];
	        this.source = source["source"];
	        this.message = source["message"];
	    }
	}

}

export namespace pipeline {

	export class Job {
	    taskID: string;
	    title: string;
	    videoPath: string;
	    source?: string;
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
	        this.source = source["source"];
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

	export class BedrockSettings {
	    region: string;
	    accessKey: string;
	    secretKey: string;
	    model: string;
	    proxyURL?: string;

	    static createFrom(source: any = {}) {
	        return new BedrockSettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.region = source["region"];
	        this.accessKey = source["accessKey"];
	        this.secretKey = source["secretKey"];
	        this.model = source["model"];
	        this.proxyURL = source["proxyURL"];
	    }
	}
	export class GeminiSettings {
	    apiKey: string;
	    model: string;
	    proxyURL?: string;

	    static createFrom(source: any = {}) {
	        return new GeminiSettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	        this.model = source["model"];
	        this.proxyURL = source["proxyURL"];
	    }
	}
	export class AISettings {
	    provider: string;
	    gemini: GeminiSettings;
	    bedrock: BedrockSettings;

	    static createFrom(source: any = {}) {
	        return new AISettings(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.gemini = this.convertValues(source["gemini"], GeminiSettings);
	        this.bedrock = this.convertValues(source["bedrock"], BedrockSettings);
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
	export class Fix {
	    id: string;
	    segmentIndex: number;
	    original: string;
	    suggested: string;
	    reason: string;
	    type: string;

	    static createFrom(source: any = {}) {
	        return new Fix(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.segmentIndex = source["segmentIndex"];
	        this.original = source["original"];
	        this.suggested = source["suggested"];
	        this.reason = source["reason"];
	        this.type = source["type"];
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
	export class NewTerm {
	    id: string;
	    term: string;
	    wrongs: string[];
	    evidence: string;
	    confidence: number;

	    static createFrom(source: any = {}) {
	        return new NewTerm(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.term = source["term"];
	        this.wrongs = source["wrongs"];
	        this.evidence = source["evidence"];
	        this.confidence = source["confidence"];
	    }
	}
	export class ProofreadResult {
	    fixes: Fix[];
	    newTerms: NewTerm[];
	    model: string;
	    createdAt: string;

	    static createFrom(source: any = {}) {
	        return new ProofreadResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fixes = this.convertValues(source["fixes"], Fix);
	        this.newTerms = this.convertValues(source["newTerms"], NewTerm);
	        this.model = source["model"];
	        this.createdAt = source["createdAt"];
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

export namespace scribe {

	export class CertStatus {
	    installed: boolean;
	    trusted: boolean;
	    name: string;

	    static createFrom(source: any = {}) {
	        return new CertStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installed = source["installed"];
	        this.trusted = source["trusted"];
	        this.name = source["name"];
	    }
	}
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
	    systemProxyManaged: boolean;
	    systemProxyEnabled: boolean;
	    systemProxyMatched: boolean;
	    systemProxyAddr?: string;
	    systemProxyDevice?: string;
	    systemProxyExpectedAddr?: string;
	    systemProxyError?: string;

	    static createFrom(source: any = {}) {
	        return new ProxyStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.running = source["running"];
	        this.interceptorAddr = source["interceptorAddr"];
	        this.apiAddr = source["apiAddr"];
	        this.lastError = source["lastError"];
	        this.systemProxyManaged = source["systemProxyManaged"];
	        this.systemProxyEnabled = source["systemProxyEnabled"];
	        this.systemProxyMatched = source["systemProxyMatched"];
	        this.systemProxyAddr = source["systemProxyAddr"];
	        this.systemProxyDevice = source["systemProxyDevice"];
	        this.systemProxyExpectedAddr = source["systemProxyExpectedAddr"];
	        this.systemProxyError = source["systemProxyError"];
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

export namespace xiaoyuzhou {

	export class AuthStatus {
	    configured: boolean;
	    valid: boolean;

	    static createFrom(source: any = {}) {
	        return new AuthStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.configured = source["configured"];
	        this.valid = source["valid"];
	    }
	}

}

