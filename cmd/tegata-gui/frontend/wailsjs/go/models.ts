export namespace config {
	
	export class AuditConfig {
	    Enabled: boolean;
	    Server: string;
	    PrivilegedServer: string;
	    SecretKey: string;
	    CertPath: string;
	    KeyPath: string;
	    CACertPath: string;
	    EntityID: string;
	    KeyVersion: number;
	    QueueMaxEvents: number;
	    Insecure: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AuditConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Enabled = source["Enabled"];
	        this.Server = source["Server"];
	        this.PrivilegedServer = source["PrivilegedServer"];
	        this.SecretKey = source["SecretKey"];
	        this.CertPath = source["CertPath"];
	        this.KeyPath = source["KeyPath"];
	        this.CACertPath = source["CACertPath"];
	        this.EntityID = source["EntityID"];
	        this.KeyVersion = source["KeyVersion"];
	        this.QueueMaxEvents = source["QueueMaxEvents"];
	        this.Insecure = source["Insecure"];
	    }
	}
	export class Config {
	    ClipboardTimeout: number;
	    IdleTimeout: number;
	    Audit: AuditConfig;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ClipboardTimeout = source["ClipboardTimeout"];
	        this.IdleTimeout = source["IdleTimeout"];
	        this.Audit = this.convertValues(source["Audit"], AuditConfig);
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

export namespace main {
	
	export class AuditHistoryRecord {
	    hash_value: string;
	    version: number;
	
	    static createFrom(source: any = {}) {
	        return new AuditHistoryRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash_value = source["hash_value"];
	        this.version = source["version"];
	    }
	}
	export class AuditVerifyResult {
	    valid: boolean;
	    event_count: number;
	    error_detail?: string;
	
	    static createFrom(source: any = {}) {
	        return new AuditVerifyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.valid = source["valid"];
	        this.event_count = source["event_count"];
	        this.error_detail = source["error_detail"];
	    }
	}
	export class ImportResult {
	    imported: number;
	    skipped: number;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = source["imported"];
	        this.skipped = source["skipped"];
	        this.path = source["path"];
	    }
	}
	export class TOTPResult {
	    code: string;
	    remaining: number;
	
	    static createFrom(source: any = {}) {
	        return new TOTPResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.remaining = source["remaining"];
	    }
	}
	export class UpdateInfo {
	    version: string;
	    url: string;
	    notes: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.url = source["url"];
	        this.notes = source["notes"];
	    }
	}
	export class VaultLocation {
	    path: string;
	    driveName: string;
	
	    static createFrom(source: any = {}) {
	        return new VaultLocation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.driveName = source["driveName"];
	    }
	}

}

export namespace model {
	
	export class Credential {
	    id: string;
	    label: string;
	    issuer?: string;
	    type: string;
	    algorithm?: string;
	    digits?: number;
	    period?: number;
	    counter?: number;
	    secret: string;
	    tags: string[];
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    modified_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Credential(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.issuer = source["issuer"];
	        this.type = source["type"];
	        this.algorithm = source["algorithm"];
	        this.digits = source["digits"];
	        this.period = source["period"];
	        this.counter = source["counter"];
	        this.secret = source["secret"];
	        this.tags = source["tags"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.modified_at = this.convertValues(source["modified_at"], null);
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

