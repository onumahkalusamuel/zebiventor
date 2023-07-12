// Cynhyrchwyd y ffeil hon yn awtomatig. PEIDIWCH Â MODIWL
// This file is automatically generated. DO NOT EDIT
import {controllers} from '../models';
import {config} from '../models';

export function Activate(arg1:controllers.ActivateRequest):Promise<{[key: string]: any}>;

export function CheckActivation():Promise<number>;

export function CheckLogin():Promise<config.LoggedInStruct>;

export function CreateAdmin(arg1:controllers.CreateAdminRequest):Promise<{[key: string]: any}>;

export function GetActivationCode(arg1:controllers.ActivationRequest):Promise<{[key: string]: any}>;

export function InstallationCode():Promise<string>;

export function Login(arg1:controllers.LoginRequest):Promise<boolean>;

export function Setup(arg1:controllers.SetupRequest):Promise<{[key: string]: any}>;

export function StoreDetails():Promise<{[key: string]: any}>;

export function UpdateStore(arg1:controllers.SetupRequest):Promise<{[key: string]: any}>;
