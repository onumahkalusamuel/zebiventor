export namespace discovery {
	
	export class Hub {
	    hub_id: string;
	    store_name: string;
	    url: string;
	    host: string;
	
	    static createFrom(source: any = {}) {
	        return new Hub(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hub_id = source["hub_id"];
	        this.store_name = source["store_name"];
	        this.url = source["url"];
	        this.host = source["host"];
	    }
	}

}

export namespace main {
	
	export class AccountPaymentResult {
	    payment?: protocol.AccountPayment;
	    printed: boolean;
	    print_error: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountPaymentResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.payment = this.convertValues(source["payment"], protocol.AccountPayment);
	        this.printed = source["printed"];
	        this.print_error = source["print_error"];
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
	export class AppState {
	    version: string;
	    data_dir: string;
	    mode: string;
	    paired: boolean;
	    pairing: terminal.Pairing;
	    user?: terminal.User;
	    status: terminal.Status;
	    settings: Record<string, string>;
	    currency: money.Currency;
	    hub_running: boolean;
	    hub_error: string;
	    hub_port: number;
	    hub_addresses: string[];
	    has_session: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new AppState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.data_dir = source["data_dir"];
	        this.mode = source["mode"];
	        this.paired = source["paired"];
	        this.pairing = this.convertValues(source["pairing"], terminal.Pairing);
	        this.user = this.convertValues(source["user"], terminal.User);
	        this.status = this.convertValues(source["status"], terminal.Status);
	        this.settings = source["settings"];
	        this.currency = this.convertValues(source["currency"], money.Currency);
	        this.hub_running = source["hub_running"];
	        this.hub_error = source["hub_error"];
	        this.hub_port = source["hub_port"];
	        this.hub_addresses = source["hub_addresses"];
	        this.has_session = source["has_session"];
	        this.error = source["error"];
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
	export class ApprovalResult {
	    token: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new ApprovalResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.token = source["token"];
	        this.name = source["name"];
	    }
	}
	export class CloseShiftResult {
	    shift?: protocol.Shift;
	    print_error: string;
	
	    static createFrom(source: any = {}) {
	        return new CloseShiftResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.shift = this.convertValues(source["shift"], protocol.Shift);
	        this.print_error = source["print_error"];
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
	export class PrinterConfig {
	    target: printing.Target;
	    paper: string;
	    mode: string;
	    auto_print: boolean;
	    open_drawer: boolean;
	    feed_lines: number;
	    cut: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PrinterConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = this.convertValues(source["target"], printing.Target);
	        this.paper = source["paper"];
	        this.mode = source["mode"];
	        this.auto_print = source["auto_print"];
	        this.open_drawer = source["open_drawer"];
	        this.feed_lines = source["feed_lines"];
	        this.cut = source["cut"];
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
	export class Diagnostics {
	    version: string;
	    os: string;
	    mode: string;
	    data_dir: string;
	    log_file: string;
	    terminal: string;
	    hub_url: string;
	    hub_running: boolean;
	    hub_error: string;
	    sync: terminal.Status;
	    printer: PrinterConfig;
	    printers: string[];
	    printers_error: string;
	    database_bytes: Record<string, number>;
	    time: string;
	
	    static createFrom(source: any = {}) {
	        return new Diagnostics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.os = source["os"];
	        this.mode = source["mode"];
	        this.data_dir = source["data_dir"];
	        this.log_file = source["log_file"];
	        this.terminal = source["terminal"];
	        this.hub_url = source["hub_url"];
	        this.hub_running = source["hub_running"];
	        this.hub_error = source["hub_error"];
	        this.sync = this.convertValues(source["sync"], terminal.Status);
	        this.printer = this.convertValues(source["printer"], PrinterConfig);
	        this.printers = source["printers"];
	        this.printers_error = source["printers_error"];
	        this.database_bytes = source["database_bytes"];
	        this.time = source["time"];
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
	
	export class ReturnResult {
	    return?: protocol.Return;
	    printed: boolean;
	    print_error: string;
	
	    static createFrom(source: any = {}) {
	        return new ReturnResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.return = this.convertValues(source["return"], protocol.Return);
	        this.printed = source["printed"];
	        this.print_error = source["print_error"];
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
	export class SaleResult {
	    sale?: protocol.Sale;
	    printed: boolean;
	    print_error: string;
	
	    static createFrom(source: any = {}) {
	        return new SaleResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sale = this.convertValues(source["sale"], protocol.Sale);
	        this.printed = source["printed"];
	        this.print_error = source["print_error"];
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
	export class SetupHostInput {
	    store_name: string;
	    business_type: string;
	    location_name: string;
	    address: string;
	    phone: string;
	    currency_code: string;
	    currency_symbol: string;
	    admin_name: string;
	    admin_username: string;
	    admin_password: string;
	    terminal_name: string;
	    port: number;
	    demo: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SetupHostInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.store_name = source["store_name"];
	        this.business_type = source["business_type"];
	        this.location_name = source["location_name"];
	        this.address = source["address"];
	        this.phone = source["phone"];
	        this.currency_code = source["currency_code"];
	        this.currency_symbol = source["currency_symbol"];
	        this.admin_name = source["admin_name"];
	        this.admin_username = source["admin_username"];
	        this.admin_password = source["admin_password"];
	        this.terminal_name = source["terminal_name"];
	        this.port = source["port"];
	        this.demo = source["demo"];
	    }
	}

}

export namespace money {
	
	export class Currency {
	    code: string;
	    symbol: string;
	    digits: number;
	
	    static createFrom(source: any = {}) {
	        return new Currency(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.symbol = source["symbol"];
	        this.digits = source["digits"];
	    }
	}

}

export namespace printing {
	
	export class Target {
	    kind: string;
	    address: string;
	
	    static createFrom(source: any = {}) {
	        return new Target(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.address = source["address"];
	    }
	}

}

export namespace protocol {
	
	export class AccountPayment {
	    id: string;
	    payment_no: string;
	    customer_id: string;
	    customer_name: string;
	    terminal_id: string;
	    shift_id: string;
	    method_id: string;
	    method_name: string;
	    kind: string;
	    amount_minor: number;
	    reference: string;
	    note: string;
	    user_id: string;
	    user_name: string;
	    balance_before_minor: number;
	    created_at: number;
	    synced_at: number;
	
	    static createFrom(source: any = {}) {
	        return new AccountPayment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.payment_no = source["payment_no"];
	        this.customer_id = source["customer_id"];
	        this.customer_name = source["customer_name"];
	        this.terminal_id = source["terminal_id"];
	        this.shift_id = source["shift_id"];
	        this.method_id = source["method_id"];
	        this.method_name = source["method_name"];
	        this.kind = source["kind"];
	        this.amount_minor = source["amount_minor"];
	        this.reference = source["reference"];
	        this.note = source["note"];
	        this.user_id = source["user_id"];
	        this.user_name = source["user_name"];
	        this.balance_before_minor = source["balance_before_minor"];
	        this.created_at = source["created_at"];
	        this.synced_at = source["synced_at"];
	    }
	}
	export class CashMovement {
	    id: string;
	    shift_id: string;
	    kind: string;
	    amount_minor: number;
	    reason: string;
	    user_id: string;
	    user_name: string;
	    approved_by: string;
	    created_at: number;
	
	    static createFrom(source: any = {}) {
	        return new CashMovement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.shift_id = source["shift_id"];
	        this.kind = source["kind"];
	        this.amount_minor = source["amount_minor"];
	        this.reason = source["reason"];
	        this.user_id = source["user_id"];
	        this.user_name = source["user_name"];
	        this.approved_by = source["approved_by"];
	        this.created_at = source["created_at"];
	    }
	}
	export class Category {
	    id: string;
	    parent_id: string;
	    name: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.parent_id = source["parent_id"];
	        this.name = source["name"];
	        this.active = source["active"];
	    }
	}
	export class Customer {
	    id: string;
	    name: string;
	    phone: string;
	    email: string;
	    notes: string;
	    active: boolean;
	    balance_minor: number;
	    updated_at: number;
	
	    static createFrom(source: any = {}) {
	        return new Customer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.phone = source["phone"];
	        this.email = source["email"];
	        this.notes = source["notes"];
	        this.active = source["active"];
	        this.balance_minor = source["balance_minor"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class HubInfo {
	    hub_id: string;
	    store_name: string;
	    version: string;
	    protocol_version: number;
	    setup_complete: boolean;
	    rev: number;
	    server_time: number;
	
	    static createFrom(source: any = {}) {
	        return new HubInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hub_id = source["hub_id"];
	        this.store_name = source["store_name"];
	        this.version = source["version"];
	        this.protocol_version = source["protocol_version"];
	        this.setup_complete = source["setup_complete"];
	        this.rev = source["rev"];
	        this.server_time = source["server_time"];
	    }
	}
	export class MethodTotal {
	    method_name: string;
	    kind: string;
	    sales_minor: number;
	    refunds_minor: number;
	    payments_minor: number;
	    net_minor: number;
	
	    static createFrom(source: any = {}) {
	        return new MethodTotal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.method_name = source["method_name"];
	        this.kind = source["kind"];
	        this.sales_minor = source["sales_minor"];
	        this.refunds_minor = source["refunds_minor"];
	        this.payments_minor = source["payments_minor"];
	        this.net_minor = source["net_minor"];
	    }
	}
	export class PairingCode {
	    code: string;
	    expires_at: number;
	
	    static createFrom(source: any = {}) {
	        return new PairingCode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.expires_at = source["expires_at"];
	    }
	}
	export class PaymentMethod {
	    id: string;
	    name: string;
	    kind: string;
	    requires_reference: boolean;
	    opens_drawer: boolean;
	    sort_order: number;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PaymentMethod(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.requires_reference = source["requires_reference"];
	        this.opens_drawer = source["opens_drawer"];
	        this.sort_order = source["sort_order"];
	        this.active = source["active"];
	    }
	}
	export class Product {
	    id: string;
	    name: string;
	    category_id: string;
	    description: string;
	    attributes: Record<string, string>;
	    unit: string;
	    allow_fraction: boolean;
	    track_stock: boolean;
	    tax_rate_bp: number;
	    active: boolean;
	    updated_at: number;
	
	    static createFrom(source: any = {}) {
	        return new Product(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category_id = source["category_id"];
	        this.description = source["description"];
	        this.attributes = source["attributes"];
	        this.unit = source["unit"];
	        this.allow_fraction = source["allow_fraction"];
	        this.track_stock = source["track_stock"];
	        this.tax_rate_bp = source["tax_rate_bp"];
	        this.active = source["active"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class VariantInput {
	    id: string;
	    product_id: string;
	    name: string;
	    sku: string;
	    price_minor: number;
	    cost_minor: number;
	    active: boolean;
	    reorder_level_milli: number;
	    barcodes: string[];
	    opening_qty_milli: number;
	    opening_location_id: string;
	
	    static createFrom(source: any = {}) {
	        return new VariantInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.product_id = source["product_id"];
	        this.name = source["name"];
	        this.sku = source["sku"];
	        this.price_minor = source["price_minor"];
	        this.cost_minor = source["cost_minor"];
	        this.active = source["active"];
	        this.reorder_level_milli = source["reorder_level_milli"];
	        this.barcodes = source["barcodes"];
	        this.opening_qty_milli = source["opening_qty_milli"];
	        this.opening_location_id = source["opening_location_id"];
	    }
	}
	export class ProductInput {
	    product: Product;
	    variants: VariantInput[];
	
	    static createFrom(source: any = {}) {
	        return new ProductInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.product = this.convertValues(source["product"], Product);
	        this.variants = this.convertValues(source["variants"], VariantInput);
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
	export class SalePayment {
	    id: string;
	    method_id: string;
	    method_name: string;
	    kind: string;
	    amount_minor: number;
	    reference: string;
	
	    static createFrom(source: any = {}) {
	        return new SalePayment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.method_id = source["method_id"];
	        this.method_name = source["method_name"];
	        this.kind = source["kind"];
	        this.amount_minor = source["amount_minor"];
	        this.reference = source["reference"];
	    }
	}
	export class ReturnLine {
	    id: string;
	    sale_line_id: string;
	    variant_id: string;
	    product_name: string;
	    variant_name: string;
	    sku: string;
	    qty_milli: number;
	    tax_minor: number;
	    total_minor: number;
	    restock: boolean;
	    track_stock: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ReturnLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sale_line_id = source["sale_line_id"];
	        this.variant_id = source["variant_id"];
	        this.product_name = source["product_name"];
	        this.variant_name = source["variant_name"];
	        this.sku = source["sku"];
	        this.qty_milli = source["qty_milli"];
	        this.tax_minor = source["tax_minor"];
	        this.total_minor = source["total_minor"];
	        this.restock = source["restock"];
	        this.track_stock = source["track_stock"];
	    }
	}
	export class Return {
	    id: string;
	    return_no: string;
	    sale_id: string;
	    sale_receipt_no: string;
	    terminal_id: string;
	    location_id: string;
	    user_id: string;
	    user_name: string;
	    approved_by: string;
	    shift_id: string;
	    customer_id: string;
	    customer_name: string;
	    reason: string;
	    tax_minor: number;
	    total_minor: number;
	    created_at: number;
	    synced_at: number;
	    lines: ReturnLine[];
	    refunds: SalePayment[];
	
	    static createFrom(source: any = {}) {
	        return new Return(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.return_no = source["return_no"];
	        this.sale_id = source["sale_id"];
	        this.sale_receipt_no = source["sale_receipt_no"];
	        this.terminal_id = source["terminal_id"];
	        this.location_id = source["location_id"];
	        this.user_id = source["user_id"];
	        this.user_name = source["user_name"];
	        this.approved_by = source["approved_by"];
	        this.shift_id = source["shift_id"];
	        this.customer_id = source["customer_id"];
	        this.customer_name = source["customer_name"];
	        this.reason = source["reason"];
	        this.tax_minor = source["tax_minor"];
	        this.total_minor = source["total_minor"];
	        this.created_at = source["created_at"];
	        this.synced_at = source["synced_at"];
	        this.lines = this.convertValues(source["lines"], ReturnLine);
	        this.refunds = this.convertValues(source["refunds"], SalePayment);
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
	
	export class SaleLine {
	    id: string;
	    line_no: number;
	    variant_id: string;
	    product_name: string;
	    variant_name: string;
	    sku: string;
	    qty_milli: number;
	    unit_price_minor: number;
	    discount_minor: number;
	    tax_minor: number;
	    total_minor: number;
	    track_stock: boolean;
	    original_price_minor: number;
	    tax_rate_bp: number;
	
	    static createFrom(source: any = {}) {
	        return new SaleLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.line_no = source["line_no"];
	        this.variant_id = source["variant_id"];
	        this.product_name = source["product_name"];
	        this.variant_name = source["variant_name"];
	        this.sku = source["sku"];
	        this.qty_milli = source["qty_milli"];
	        this.unit_price_minor = source["unit_price_minor"];
	        this.discount_minor = source["discount_minor"];
	        this.tax_minor = source["tax_minor"];
	        this.total_minor = source["total_minor"];
	        this.track_stock = source["track_stock"];
	        this.original_price_minor = source["original_price_minor"];
	        this.tax_rate_bp = source["tax_rate_bp"];
	    }
	}
	export class Sale {
	    id: string;
	    receipt_no: string;
	    terminal_id: string;
	    location_id: string;
	    user_id: string;
	    user_name: string;
	    customer_id: string;
	    status: string;
	    subtotal_minor: number;
	    discount_minor: number;
	    tax_minor: number;
	    total_minor: number;
	    paid_minor: number;
	    change_minor: number;
	    note: string;
	    shift_id: string;
	    approved_by: string;
	    customer_name: string;
	    tax_mode: string;
	    created_at: number;
	    synced_at: number;
	    lines: SaleLine[];
	    payments: SalePayment[];
	
	    static createFrom(source: any = {}) {
	        return new Sale(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.receipt_no = source["receipt_no"];
	        this.terminal_id = source["terminal_id"];
	        this.location_id = source["location_id"];
	        this.user_id = source["user_id"];
	        this.user_name = source["user_name"];
	        this.customer_id = source["customer_id"];
	        this.status = source["status"];
	        this.subtotal_minor = source["subtotal_minor"];
	        this.discount_minor = source["discount_minor"];
	        this.tax_minor = source["tax_minor"];
	        this.total_minor = source["total_minor"];
	        this.paid_minor = source["paid_minor"];
	        this.change_minor = source["change_minor"];
	        this.note = source["note"];
	        this.shift_id = source["shift_id"];
	        this.approved_by = source["approved_by"];
	        this.customer_name = source["customer_name"];
	        this.tax_mode = source["tax_mode"];
	        this.created_at = source["created_at"];
	        this.synced_at = source["synced_at"];
	        this.lines = this.convertValues(source["lines"], SaleLine);
	        this.payments = this.convertValues(source["payments"], SalePayment);
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
	
	
	export class ShiftSummary {
	    sales_count: number;
	    items_milli: number;
	    gross_minor: number;
	    discount_minor: number;
	    tax_minor: number;
	    net_sales_minor: number;
	    returns_count: number;
	    returns_minor: number;
	    by_method: MethodTotal[];
	    opening_float_minor: number;
	    cash_sales_minor: number;
	    cash_refunds_minor: number;
	    paid_in_minor: number;
	    paid_out_minor: number;
	    account_payments_count: number;
	    account_payments_minor: number;
	    cash_account_payments_minor: number;
	    expected_cash_minor: number;
	    counted_cash_minor: number;
	    variance_minor: number;
	    closed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ShiftSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sales_count = source["sales_count"];
	        this.items_milli = source["items_milli"];
	        this.gross_minor = source["gross_minor"];
	        this.discount_minor = source["discount_minor"];
	        this.tax_minor = source["tax_minor"];
	        this.net_sales_minor = source["net_sales_minor"];
	        this.returns_count = source["returns_count"];
	        this.returns_minor = source["returns_minor"];
	        this.by_method = this.convertValues(source["by_method"], MethodTotal);
	        this.opening_float_minor = source["opening_float_minor"];
	        this.cash_sales_minor = source["cash_sales_minor"];
	        this.cash_refunds_minor = source["cash_refunds_minor"];
	        this.paid_in_minor = source["paid_in_minor"];
	        this.paid_out_minor = source["paid_out_minor"];
	        this.account_payments_count = source["account_payments_count"];
	        this.account_payments_minor = source["account_payments_minor"];
	        this.cash_account_payments_minor = source["cash_account_payments_minor"];
	        this.expected_cash_minor = source["expected_cash_minor"];
	        this.counted_cash_minor = source["counted_cash_minor"];
	        this.variance_minor = source["variance_minor"];
	        this.closed = source["closed"];
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
	export class Shift {
	    id: string;
	    terminal_id: string;
	    location_id: string;
	    status: string;
	    opened_by: string;
	    opened_by_name: string;
	    opened_at: number;
	    opening_float_minor: number;
	    closed_by: string;
	    closed_by_name: string;
	    closed_at: number;
	    counted_cash_minor: number;
	    note: string;
	    summary?: ShiftSummary;
	
	    static createFrom(source: any = {}) {
	        return new Shift(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.terminal_id = source["terminal_id"];
	        this.location_id = source["location_id"];
	        this.status = source["status"];
	        this.opened_by = source["opened_by"];
	        this.opened_by_name = source["opened_by_name"];
	        this.opened_at = source["opened_at"];
	        this.opening_float_minor = source["opening_float_minor"];
	        this.closed_by = source["closed_by"];
	        this.closed_by_name = source["closed_by_name"];
	        this.closed_at = source["closed_at"];
	        this.counted_cash_minor = source["counted_cash_minor"];
	        this.note = source["note"];
	        this.summary = this.convertValues(source["summary"], ShiftSummary);
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
	
	export class StockAdjustRequest {
	    variant_id: string;
	    location_id: string;
	    delta_milli?: number;
	    count_milli?: number;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new StockAdjustRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variant_id = source["variant_id"];
	        this.location_id = source["location_id"];
	        this.delta_milli = source["delta_milli"];
	        this.count_milli = source["count_milli"];
	        this.note = source["note"];
	    }
	}
	export class TerminalInfo {
	    id: string;
	    name: string;
	    location_id: string;
	    location_name: string;
	    receipt_prefix: string;
	    active: boolean;
	    paired_at: number;
	    last_seen_at: number;
	    last_ip: string;
	    app_version: string;
	
	    static createFrom(source: any = {}) {
	        return new TerminalInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.location_id = source["location_id"];
	        this.location_name = source["location_name"];
	        this.receipt_prefix = source["receipt_prefix"];
	        this.active = source["active"];
	        this.paired_at = source["paired_at"];
	        this.last_seen_at = source["last_seen_at"];
	        this.last_ip = source["last_ip"];
	        this.app_version = source["app_version"];
	    }
	}
	export class UserInput {
	    id: string;
	    name: string;
	    username: string;
	    role: string;
	    password: string;
	    pin: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UserInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.username = source["username"];
	        this.role = source["role"];
	        this.password = source["password"];
	        this.pin = source["pin"];
	        this.active = source["active"];
	    }
	}

}

export namespace terminal {
	
	export class AccountPaymentInput {
	    customer_id: string;
	    method_id: string;
	    amount_minor: number;
	    reference: string;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountPaymentInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.customer_id = source["customer_id"];
	        this.method_id = source["method_id"];
	        this.amount_minor = source["amount_minor"];
	        this.reference = source["reference"];
	        this.note = source["note"];
	    }
	}
	export class CartLine {
	    variant_id: string;
	    qty_milli: number;
	    price_override?: number;
	
	    static createFrom(source: any = {}) {
	        return new CartLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variant_id = source["variant_id"];
	        this.qty_milli = source["qty_milli"];
	        this.price_override = source["price_override"];
	    }
	}
	export class HeldSale {
	    id: string;
	    label: string;
	    user_name: string;
	    item_count: number;
	    total_minor: number;
	    payload: string;
	    created_at: number;
	
	    static createFrom(source: any = {}) {
	        return new HeldSale(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.user_name = source["user_name"];
	        this.item_count = source["item_count"];
	        this.total_minor = source["total_minor"];
	        this.payload = source["payload"];
	        this.created_at = source["created_at"];
	    }
	}
	export class Item {
	    variant_id: string;
	    product_id: string;
	    name: string;
	    variant_name: string;
	    sku: string;
	    price_minor: number;
	    unit: string;
	    allow_fraction: boolean;
	    track_stock: boolean;
	    stock_milli: number;
	    category_id: string;
	    attributes: Record<string, string>;
	    tax_rate_bp: number;
	    pack_qty_milli: number;
	
	    static createFrom(source: any = {}) {
	        return new Item(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variant_id = source["variant_id"];
	        this.product_id = source["product_id"];
	        this.name = source["name"];
	        this.variant_name = source["variant_name"];
	        this.sku = source["sku"];
	        this.price_minor = source["price_minor"];
	        this.unit = source["unit"];
	        this.allow_fraction = source["allow_fraction"];
	        this.track_stock = source["track_stock"];
	        this.stock_milli = source["stock_milli"];
	        this.category_id = source["category_id"];
	        this.attributes = source["attributes"];
	        this.tax_rate_bp = source["tax_rate_bp"];
	        this.pack_qty_milli = source["pack_qty_milli"];
	    }
	}
	export class Pairing {
	    hub_url: string;
	    hub_id: string;
	    store_name: string;
	    terminal_id: string;
	    terminal_name: string;
	    receipt_prefix: string;
	    location_id: string;
	
	    static createFrom(source: any = {}) {
	        return new Pairing(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hub_url = source["hub_url"];
	        this.hub_id = source["hub_id"];
	        this.store_name = source["store_name"];
	        this.terminal_id = source["terminal_id"];
	        this.terminal_name = source["terminal_name"];
	        this.receipt_prefix = source["receipt_prefix"];
	        this.location_id = source["location_id"];
	    }
	}
	export class PaymentInput {
	    method_id: string;
	    amount_minor: number;
	    reference: string;
	
	    static createFrom(source: any = {}) {
	        return new PaymentInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.method_id = source["method_id"];
	        this.amount_minor = source["amount_minor"];
	        this.reference = source["reference"];
	    }
	}
	export class VariantDetail {
	    id: string;
	    product_id: string;
	    name: string;
	    sku: string;
	    price_minor: number;
	    cost_minor: number;
	    active: boolean;
	    reorder_level_milli: number;
	    barcodes: string[];
	    stock_milli: number;
	
	    static createFrom(source: any = {}) {
	        return new VariantDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.product_id = source["product_id"];
	        this.name = source["name"];
	        this.sku = source["sku"];
	        this.price_minor = source["price_minor"];
	        this.cost_minor = source["cost_minor"];
	        this.active = source["active"];
	        this.reorder_level_milli = source["reorder_level_milli"];
	        this.barcodes = source["barcodes"];
	        this.stock_milli = source["stock_milli"];
	    }
	}
	export class ProductDetail {
	    id: string;
	    name: string;
	    category_id: string;
	    description: string;
	    attributes: Record<string, string>;
	    unit: string;
	    allow_fraction: boolean;
	    track_stock: boolean;
	    tax_rate_bp: number;
	    active: boolean;
	    updated_at: number;
	    category_name: string;
	    variants: VariantDetail[];
	
	    static createFrom(source: any = {}) {
	        return new ProductDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category_id = source["category_id"];
	        this.description = source["description"];
	        this.attributes = source["attributes"];
	        this.unit = source["unit"];
	        this.allow_fraction = source["allow_fraction"];
	        this.track_stock = source["track_stock"];
	        this.tax_rate_bp = source["tax_rate_bp"];
	        this.active = source["active"];
	        this.updated_at = source["updated_at"];
	        this.category_name = source["category_name"];
	        this.variants = this.convertValues(source["variants"], VariantDetail);
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
	export class ReturnLineInput {
	    sale_line_id: string;
	    qty_milli: number;
	    restock: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ReturnLineInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sale_line_id = source["sale_line_id"];
	        this.qty_milli = source["qty_milli"];
	        this.restock = source["restock"];
	    }
	}
	export class ReturnInput {
	    receipt_no: string;
	    lines: ReturnLineInput[];
	    refunds: PaymentInput[];
	    reason: string;
	    approval_token: string;
	
	    static createFrom(source: any = {}) {
	        return new ReturnInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.receipt_no = source["receipt_no"];
	        this.lines = this.convertValues(source["lines"], ReturnLineInput);
	        this.refunds = this.convertValues(source["refunds"], PaymentInput);
	        this.reason = source["reason"];
	        this.approval_token = source["approval_token"];
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
	
	export class ReturnableLine {
	    id: string;
	    line_no: number;
	    variant_id: string;
	    product_name: string;
	    variant_name: string;
	    sku: string;
	    qty_milli: number;
	    unit_price_minor: number;
	    discount_minor: number;
	    tax_minor: number;
	    total_minor: number;
	    track_stock: boolean;
	    original_price_minor: number;
	    tax_rate_bp: number;
	    returned_milli: number;
	    returnable_milli: number;
	    paid_minor: number;
	
	    static createFrom(source: any = {}) {
	        return new ReturnableLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.line_no = source["line_no"];
	        this.variant_id = source["variant_id"];
	        this.product_name = source["product_name"];
	        this.variant_name = source["variant_name"];
	        this.sku = source["sku"];
	        this.qty_milli = source["qty_milli"];
	        this.unit_price_minor = source["unit_price_minor"];
	        this.discount_minor = source["discount_minor"];
	        this.tax_minor = source["tax_minor"];
	        this.total_minor = source["total_minor"];
	        this.track_stock = source["track_stock"];
	        this.original_price_minor = source["original_price_minor"];
	        this.tax_rate_bp = source["tax_rate_bp"];
	        this.returned_milli = source["returned_milli"];
	        this.returnable_milli = source["returnable_milli"];
	        this.paid_minor = source["paid_minor"];
	    }
	}
	export class ReturnableSale {
	    sale: protocol.Sale;
	    lines: ReturnableLine[];
	    source: string;
	    partial: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ReturnableSale(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sale = this.convertValues(source["sale"], protocol.Sale);
	        this.lines = this.convertValues(source["lines"], ReturnableLine);
	        this.source = source["source"];
	        this.partial = source["partial"];
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
	export class SaleInput {
	    lines: CartLine[];
	    payments: PaymentInput[];
	    discount_minor: number;
	    customer_id: string;
	    note: string;
	    approval_token: string;
	
	    static createFrom(source: any = {}) {
	        return new SaleInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lines = this.convertValues(source["lines"], CartLine);
	        this.payments = this.convertValues(source["payments"], PaymentInput);
	        this.discount_minor = source["discount_minor"];
	        this.customer_id = source["customer_id"];
	        this.note = source["note"];
	        this.approval_token = source["approval_token"];
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
	export class SaleSummary {
	    id: string;
	    kind: string;
	    receipt_no: string;
	    user_name: string;
	    customer: string;
	    total_minor: number;
	    items: number;
	    methods: string;
	    created_at: number;
	    synced: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SaleSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.receipt_no = source["receipt_no"];
	        this.user_name = source["user_name"];
	        this.customer = source["customer"];
	        this.total_minor = source["total_minor"];
	        this.items = source["items"];
	        this.methods = source["methods"];
	        this.created_at = source["created_at"];
	        this.synced = source["synced"];
	    }
	}
	export class StaffUser {
	    id: string;
	    name: string;
	    username: string;
	    role: string;
	    active: boolean;
	    has_pin: boolean;
	
	    static createFrom(source: any = {}) {
	        return new StaffUser(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.username = source["username"];
	        this.role = source["role"];
	        this.active = source["active"];
	        this.has_pin = source["has_pin"];
	    }
	}
	export class Status {
	    paired: boolean;
	    online: boolean;
	    revoked: boolean;
	    hub_url: string;
	    last_sync_at: number;
	    last_error: string;
	    pending: number;
	    rejected: number;
	    rev: number;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.paired = source["paired"];
	        this.online = source["online"];
	        this.revoked = source["revoked"];
	        this.hub_url = source["hub_url"];
	        this.last_sync_at = source["last_sync_at"];
	        this.last_error = source["last_error"];
	        this.pending = source["pending"];
	        this.rejected = source["rejected"];
	        this.rev = source["rev"];
	    }
	}
	export class User {
	    id: string;
	    name: string;
	    username: string;
	    role: string;
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.username = source["username"];
	        this.role = source["role"];
	    }
	}

}

