export namespace backend {
	
	export class LaunchOptions {
	    binPath?: string;
	    data?: string;
	    file?: string;
	    host?: string;
	    logLevel?: string;
	    logFilename?: string;
	    httpDebug?: boolean;
	    httpEnableTokenAuth?: boolean;
	    mqttEnableTokenAuth?: boolean;
	    mqttEnableTls?: boolean;
	    jwtAtExpire?: string;
	    jwtRtExpire?: string;
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
	        this.httpDebug = source["httpDebug"];
	        this.httpEnableTokenAuth = source["httpEnableTokenAuth"];
	        this.mqttEnableTokenAuth = source["mqttEnableTokenAuth"];
	        this.mqttEnableTls = source["mqttEnableTls"];
	        this.jwtAtExpire = source["jwtAtExpire"];
	        this.jwtRtExpire = source["jwtRtExpire"];
	        this.experiment = source["experiment"];
	    }
	}

}

