import { EventEmitter } from "events";
import { Socket } from "net";
import { v4 } from "uuid";
import {
  marshalMessage,
  marshalRequest,
  marshalResponse,
  unmarshalMessage,
  unmarshalRequest,
  unmarshalResponse,
} from "./messages";

export const ErrorCallCancelled = "call timed out";

export interface ILocalContext {
  remoteID: string;
}

export interface ILocal {
  [k: string]: (ctx: ILocalContext, ...rest: any) => Promise<any>;
}

export type IRemoteContext =
  | undefined
  | {
      signal?: AbortSignal;
    };

export interface IRemote {
  [k: string]: (ctx: IRemoteContext, ...rest: any) => Promise<any>;
}

interface ICallResponse {
  value?: any;
  err: string;
}

/**
 * Expose local functions and link remote ones to a WebSocket
 * @returns Remote functions
 */
export const linkWebSocket = <L extends ILocal, R extends IRemote, T>(
  socket: WebSocket,

  local: L,
  remote: R,

  stringify: (value: any) => string,
  parse: (text: string) => any,

  stringifyNested: (value: any) => T,
  parseNested: (text: T) => any
) => {
  const broker = new EventEmitter();

  const r = { ...remote };
  // eslint-disable-next-line no-restricted-syntax, guard-for-in
  for (const functionName in r) {
    (r as any)[functionName] = async (ctx: IRemoteContext, ...rest: any[]) =>
      new Promise((res, rej) => {
        if (ctx?.signal?.aborted) {
          rej(new Error(ErrorCallCancelled));

          return;
        }

        const id = v4();

        const abortListener = () => {
          ctx?.signal?.removeEventListener("abort", abortListener);

          const callResponse: ICallResponse = {
            err: ErrorCallCancelled,
          };

          broker.emit(`rpc:${id}`, callResponse);
        };
        ctx?.signal?.addEventListener("abort", abortListener);

        const returnListener = ({ value, err }: ICallResponse) => {
          broker.removeListener(`rpc:${id}`, returnListener);

          if (err) {
            rej(new Error(err));
          } else {
            res(value);
          }
        };
        broker.addListener(`rpc:${id}`, returnListener);

        socket.send(
          marshalMessage<T>(
            marshalRequest<T>(id, functionName, rest, stringifyNested),
            undefined,
            stringify
          )
        );
      });
  }

  socket.addEventListener("message", async (event) => {
    const msg = unmarshalMessage<T>(event.data as string, parse);

    if (msg.request) {
      const { call, functionName, args } = unmarshalRequest<T>(
        msg.request,
        parseNested
      );

      let res: T;
      try {
        const ctx: ILocalContext = { remoteID: "1" }; // TODO: Use remote-unique ID here

        const rv = await (local as any)[functionName](ctx, ...args);

        res = marshalResponse<T>(call, rv, "", stringifyNested);
      } catch (e) {
        res = marshalResponse<T>(
          call,
          undefined,
          (e as Error).message,
          stringifyNested
        );
      }

      socket.send(marshalMessage<T>(undefined, res, stringify));
    } else if (msg.response) {
      const { call, value, err } = unmarshalResponse<T>(
        msg.response,
        parseNested
      );

      const callResponse: ICallResponse = {
        value,
        err,
      };

      broker.emit(`rpc:${call}`, callResponse);
    }
  });

  return r;
};

/**
 * Expose local functions and link remote ones to a TCP socket
 * @param socket TCP socket to use
 * @param local Local functions to expose
 * @returns Remote functions
 */
export const linkTCPSocket = <R, T>(
  socket: Socket,

  local: any,
  remote: R,

  timeout: number,

  stringify: (value: any) => string,
  parse: (text: string) => any,

  stringifyNested: (value: any) => T,
  parseNested: (text: T) => any
) => {
  const broker = new EventEmitter();

  const r = { ...remote };
  // eslint-disable-next-line no-restricted-syntax, guard-for-in
  for (const functionName in r) {
    (r as any)[functionName] = async (...args: any[]) =>
      new Promise((res, rej) => {
        const id = v4();

        const handleReturn = ({ value, err }: ICallResponse) => {
          if (err) {
            rej(new Error(err));
          } else {
            res(value);
          }

          broker.removeListener(`rpc:${id}`, handleReturn);
        };

        const t = setTimeout(() => {
          const callResponse: ICallResponse = {
            err: ErrorCallCancelled,
          };

          broker.emit(`rpc:${id}`, callResponse);
        }, timeout);

        broker.addListener(`rpc:${id}`, (e) => {
          clearTimeout(t);

          handleReturn(e);
        });

        socket.write(
          marshalMessage<T>(
            marshalRequest<T>(id, functionName, args, stringifyNested),
            undefined,
            stringify
          )
        );
      });
  }

  socket.on("data", async (data) => {
    const msg = unmarshalMessage<T>(data.toString(), parse);

    if (msg.request) {
      const { call, functionName, args } = unmarshalRequest<T>(
        msg.request,
        parseNested
      );

      let res: T;
      try {
        const rv = await (local as any)[functionName](...args);

        res = marshalResponse<T>(call, rv, "", stringifyNested);
      } catch (e) {
        res = marshalResponse<T>(
          call,
          undefined,
          (e as Error).message,
          stringifyNested
        );
      }

      socket.write(marshalMessage<T>(undefined, res, stringify));
    } else if (msg.response) {
      const { call, value, err } = unmarshalResponse<T>(
        msg.response,
        parseNested
      );

      const callResponse: ICallResponse = {
        value,
        err,
      };

      broker.emit(`rpc:${call}`, callResponse);
    }
  });

  return r;
};
