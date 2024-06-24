export namespace backend {
	
	export class LaunchOptions {
	    binPath?: string;
	    data?: string;
	    file?: string;
	    host?: string;
	    logLevel?: string;
	    logFilename?: string;
	    experiment?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LaunchOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.binPath = source["binPath"];
	        this.data = source["data"];
	        this.file = source["file"];
	        this.host = source["host"];
	        this.logLevel = source["logLevel"];
	        this.logFilename = source["logFilename"];
	        this.experiment = source["experiment"];
	    }
	}

}

