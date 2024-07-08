// Cynhyrchwyd y ffeil hon yn awtomatig. PEIDIWCH Â MODIWL
// This file is automatically generated. DO NOT EDIT
import {backend} from '../models';
import {io} from '../models';

export function DoClearLog():Promise<void>;

export function DoCopyLog():Promise<void>;

export function DoFrontendReady():Promise<void>;

export function DoGetFlags():Promise<void>;

export function DoGetLaunchOptions():Promise<backend.LaunchOptions>;

export function DoGetNeoCatLauncher():Promise<backend.NeoCatOptions>;

export function DoGetOS():Promise<string>;

export function DoGetProcessInfo():Promise<string>;

export function DoGetRecentDirList():Promise<Array<string>>;

export function DoGetRecentFileList():Promise<Array<string>>;

export function DoGetTheme():Promise<string>;

export function DoOpenBrowser():Promise<void>;

export function DoRevealConfig():Promise<void>;

export function DoRevealNeoBin():Promise<void>;

export function DoSaveLog():Promise<void>;

export function DoSelectDirectory(arg1:string):Promise<string>;

export function DoSetLaunchOptions(arg1:backend.LaunchOptions):Promise<void>;

export function DoSetNeoCatLauncher(arg1:backend.NeoCatOptions):Promise<void>;

export function DoSetTheme(arg1:string):Promise<void>;

export function DoStartNeoCat():Promise<void>;

export function DoStartServer():Promise<void>;

export function DoStopNeoCat():Promise<void>;

export function DoStopServer():Promise<void>;

export function DoVersion():Promise<void>;

export function NewLogWriter():Promise<io.Writer>;
