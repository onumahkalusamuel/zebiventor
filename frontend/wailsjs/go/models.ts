export namespace config {
	
	export class LoggedInStruct {
	    id: string;
	    role: number;
	    name: string;
	    username: string;
	
	    static createFrom(source: any = {}) {
	        return new LoggedInStruct(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.role = source["role"];
	        this.name = source["name"];
	        this.username = source["username"];
	    }
	}

}

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
	export class CustomerSearchParams {
	    query: string;
	    customer_id: string;
	
	    static createFrom(source: any = {}) {
	        return new CustomerSearchParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.customer_id = source["customer_id"];
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
	export class ProductSearchParams {
	    query: string;
	    category_id: string;
	
	    static createFrom(source: any = {}) {
	        return new ProductSearchParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.category_id = source["category_id"];
	    }
	}
	export class SalesSearch {
	    customer_id: string;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new SalesSearch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.customer_id = source["customer_id"];
	        this.created_at = source["created_at"];
	    }
	}
	export class SetupRequest {
	    name: string;
	    address: string;
	    email: string;
	    phone: string;
	
	    static createFrom(source: any = {}) {
	        return new SetupRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.address = source["address"];
	        this.email = source["email"];
	        this.phone = source["phone"];
	    }
	}
	export class StaffSearchParams {
	    query: string;
	    staff_id: string;
	
	    static createFrom(source: any = {}) {
	        return new StaffSearchParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.staff_id = source["staff_id"];
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
	
	export class Product {
	    id: string;
	    // Go type: time
	    created_at: any;
	    category_id: string;
	    code: string;
	    name: string;
	    price: number;
	    stock_quantity: number;
	    category?: Category;
	
	    static createFrom(source: any = {}) {
	        return new Product(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.category_id = source["category_id"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.price = source["price"];
	        this.stock_quantity = source["stock_quantity"];
	        this.category = this.convertValues(source["category"], Category);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice) {
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
	export class Category {
	    id: string;
	    // Go type: time
	    created_at: any;
	    name: string;
	    products: Product[];
	
	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.name = source["name"];
	        this.products = this.convertValues(source["products"], Product);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice) {
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
	export class InvoiceDetails {
	    product_name: string;
	    product_id: string;
	    product_code: string;
	    unit_price: number;
	    qty: number;
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new InvoiceDetails(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.product_name = source["product_name"];
	        this.product_id = source["product_id"];
	        this.product_code = source["product_code"];
	        this.unit_price = source["unit_price"];
	        this.qty = source["qty"];
	        this.total = source["total"];
	    }
	}
	export class Sale {
	    id: string;
	    // Go type: time
	    created_at: any;
	    customer_id: string;
	    details: InvoiceDetails[];
	    sub_total: number;
	    discount_amount: number;
	    grand_total: number;
	    payment_method: string;
	    payment_reference: string;
	    customer?: Customer;
	
	    static createFrom(source: any = {}) {
	        return new Sale(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.customer_id = source["customer_id"];
	        this.details = this.convertValues(source["details"], InvoiceDetails);
	        this.sub_total = source["sub_total"];
	        this.discount_amount = source["discount_amount"];
	        this.grand_total = source["grand_total"];
	        this.payment_method = source["payment_method"];
	        this.payment_reference = source["payment_reference"];
	        this.customer = this.convertValues(source["customer"], Customer);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice) {
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
	export class Customer {
	    id: string;
	    // Go type: time
	    created_at: any;
	    customer_code: string;
	    name: string;
	    phone: string;
	    email: string;
	    sex: string;
	    sales: Sale[];
	
	    static createFrom(source: any = {}) {
	        return new Customer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.customer_code = source["customer_code"];
	        this.name = source["name"];
	        this.phone = source["phone"];
	        this.email = source["email"];
	        this.sex = source["sex"];
	        this.sales = this.convertValues(source["sales"], Sale);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice) {
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
	
	
	
	export class Staff {
	    id: string;
	    // Go type: time
	    created_at: any;
	    name: string;
	    phone: string;
	    sex: string;
	    role: number;
	    username: string;
	
	    static createFrom(source: any = {}) {
	        return new Staff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.name = source["name"];
	        this.phone = source["phone"];
	        this.sex = source["sex"];
	        this.role = source["role"];
	        this.username = source["username"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice) {
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

