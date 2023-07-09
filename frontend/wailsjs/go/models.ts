export namespace controllers {
	
	export class ActivateRequest {
	    activation_code: string;
	
	    static createFrom(source: any = {}) {
	        return new ActivateRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.activation_code = source["activation_code"];
	    }
	}
	export class ActivationRequest {
	    installation_code: string;
	
	    static createFrom(source: any = {}) {
	        return new ActivationRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.installation_code = source["installation_code"];
	    }
	}
	export class CreateAdminRequest {
	    name: string;
	    username: string;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateAdminRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.username = source["username"];
	        this.password = source["password"];
	    }
	}
	export class CreateStaffRequest {
	    name: string;
	    username: string;
	    password: string;
	    phone: string;
	    role: number;
	    sex: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateStaffRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.phone = source["phone"];
	        this.role = source["role"];
	        this.sex = source["sex"];
	    }
	}
	export class LoginRequest {
	    username: string;
	    password: string;
	
	    static createFrom(source: any = {}) {
	        return new LoginRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.password = source["password"];
	    }
	}
	export class SetupRequest {
	    name: string;
	    address: string;
	    email: string;
	    phone: string;
	    logo: string;
	
	    static createFrom(source: any = {}) {
	        return new SetupRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.address = source["address"];
	        this.email = source["email"];
	        this.phone = source["phone"];
	        this.logo = source["logo"];
	    }
	}
	export class UpdatePasswordRequest {
	    old_password: string;
	    new_password: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdatePasswordRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.old_password = source["old_password"];
	        this.new_password = source["new_password"];
	    }
	}
	export class UpdateStaffRequest {
	    name: string;
	    username: string;
	    password: string;
	    phone: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateStaffRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.phone = source["phone"];
	    }
	}

}

export namespace models {
	
	export class Staff {
	
	
	    static createFrom(source: any = {}) {
	        return new Staff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

