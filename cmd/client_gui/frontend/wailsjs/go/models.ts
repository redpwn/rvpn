export namespace common {
	
	export class ClientOptions {
	    subnets: string[];
	
	    static createFrom(source: any = {}) {
	        return new ClientOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.subnets = source["subnets"];
	    }
	}

}

export namespace main {
	
	export class WrappedReturn {
	    success: boolean;
	    data: string;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new WrappedReturn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.data = source["data"];
	        this.error = source["error"];
	    }
	}

}

