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
	
	export class CartLine {
	    variant_id: string;
	    qty_milli: number;
	
	    static createFrom(source: any = {}) {
	        return new CartLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.variant_id = source["variant_id"];
	        this.qty_milli = source["qty_milli"];
	    }
	}
	export class DayTotals {
	    count: number;
	    total_minor: number;
	    by_method: Record<string, number>;
	    unsynced: number;
	
	    static createFrom(source: any = {}) {
	        return new DayTotals(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.count = source["count"];
	        this.total_minor = source["total_minor"];
	        this.by_method = source["by_method"];
	        this.unsynced = source["unsynced"];
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
	export class SaleInput {
	    lines: CartLine[];
	    payments: PaymentInput[];
	    discount_minor: number;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new SaleInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.lines = this.convertValues(source["lines"], CartLine);
	        this.payments = this.convertValues(source["payments"], PaymentInput);
	        this.discount_minor = source["discount_minor"];
	        this.note = source["note"];
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
	    receipt_no: string;
	    user_name: string;
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
	        this.receipt_no = source["receipt_no"];
	        this.user_name = source["user_name"];
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

