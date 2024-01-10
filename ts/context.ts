export interface ILocalContext {
  remoteID: string;
}

export type IRemoteContext =
  | undefined
  | {
      signal?: AbortSignal;
    };
