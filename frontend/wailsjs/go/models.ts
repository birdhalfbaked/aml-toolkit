export namespace main {
	
	export class APIDispatchResult {
	    status: number;
	    contentType: string;
	    body: number[];
	
	    static createFrom(source: any = {}) {
	        return new APIDispatchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.contentType = source["contentType"];
	        this.body = source["body"];
	    }
	}
	export class DesktopAudioFileBytes {
	    data: number[];
	    mime: string;
	
	    static createFrom(source: any = {}) {
	        return new DesktopAudioFileBytes(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.data = source["data"];
	        this.mime = source["mime"];
	    }
	}

}

export namespace models {
	
	export class AudioFile {
	    id: number;
	    collectionId: number;
	    storedFilename: string;
	    originalName: string;
	    format: string;
	    durationMs?: number;
	    // Go type: time
	    uploadedAt: any;
	    fieldValues?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new AudioFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.collectionId = source["collectionId"];
	        this.storedFilename = source["storedFilename"];
	        this.originalName = source["originalName"];
	        this.format = source["format"];
	        this.durationMs = source["durationMs"];
	        this.uploadedAt = this.convertValues(source["uploadedAt"], null);
	        this.fieldValues = source["fieldValues"];
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

