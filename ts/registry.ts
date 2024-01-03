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

export interface IRequestResponseReader<T> {
  on(event: "request", listener: (message: T) => void): this;
  on(event: "response", listener: (message: T) => void): this;
  on(event: "close", listener: () => void): this;
}

class WebSocketRequestResponseReader<T> implements IRequestResponseReader<T> {
  private requestListener?: (message: T) => void;

  private responseListener?: (message: T) => void;

  private closeListener?: () => void;

  constructor(socket: WebSocket, parse: (text: string) => any) {
    socket.addEventListener(
      "message",
      (event: MessageEvent<string | Buffer>) => {
        const msg = unmarshalMessage<T>(event.data as string, parse);

        if (msg.request) {
          this.requestListener?.(msg.request);
        } else if (msg.response) {
          this.responseListener?.(msg.response);
        }
      }
    );

    socket.addEventListener("close", () => this.closeListener?.());
  }

  on = (
    event: "request" | "response" | "close",
    listener: ((message: T) => void) | (() => void)
  ): this => {
    if (event === "request") {
      this.requestListener = listener;
    } else if (event === "response") {
      this.responseListener = listener;
    } else if (event === "close") {
      this.closeListener = listener as () => void;
    }

    return this;
  };
}

class TCPSocketRequestResponseReader<T> implements IRequestResponseReader<T> {
  private requestListener?: (message: T) => void;

  private responseListener?: (message: T) => void;

  private closeListener?: () => void;

  constructor(socket: Socket, parse: (text: string) => any) {
    socket.addListener("data", (data: any) => {
      const msg = unmarshalMessage<T>(data.toString() as string, parse);

      if (msg.request) {
        this.requestListener?.(msg.request);
      } else if (msg.response) {
        this.responseListener?.(msg.response);
      }
    });

    socket.addListener("close", () => this.closeListener?.());
  }

  on = (
    event: "request" | "response" | "close",
    listener: (message: T) => void
  ): this => {
    if (event === "request") {
      this.requestListener = listener;
    } else if (event === "response") {
      this.responseListener = listener;
    } else if (event === "close") {
      this.closeListener = listener as () => void;
    }

    return this;
  };
}

export interface IOptions {
  onClientConnect?: (remoteID: string) => void;
  onClientDisconnect?: (remoteID: string) => void;
}

export class Registry<L extends ILocal, R extends IRemote> {
  private remotes: {
    [remoteID: string]: R;
  } = {};

  constructor(
    private local: L,
    private remote: R,
    private options?: IOptions
  ) {}

  /**
   * Expose local functions and link remote ones to a message-based transport
   * @param writeRequest Function to write a request
   * @param writeResponse Function to write a response
   * @param requestResponseReader Emitter to read requests and responses
   * @param stringify Function to marshal a message
   * @param parse Function to unmarshal a message
   * @returns Remote functions
   */
  linkMessage = <T>(
    writeRequest: (text: T) => Promise<any>,
    writeResponse: (text: T) => Promise<any>,

    requestResponseReader: IRequestResponseReader<T>,

    stringify: (value: any) => T,
    parse: (text: T) => any
  ) => {
    const broker = new EventEmitter();

    const r = { ...this.remote };
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

          writeRequest(
            marshalRequest<T>(id, functionName, rest, stringify)
          ).catch(rej);
        });
    }

    const remoteID = v4();

    requestResponseReader.on("request", async (message) => {
      const { call, functionName, args } = unmarshalRequest<T>(message, parse);

      let res: T;
      try {
        const ctx: ILocalContext = { remoteID };

        const rv = await (this.local as any)[functionName](ctx, ...args);

        res = marshalResponse<T>(call, rv, "", stringify);
      } catch (e) {
        res = marshalResponse<T>(
          call,
          undefined,
          (e as Error).message,
          stringify
        );
      }

      await writeResponse(res);
    });

    requestResponseReader.on("response", async (message) => {
      const { call, value, err } = unmarshalResponse<T>(message, parse);

      const callResponse: ICallResponse = {
        value,
        err,
      };

      broker.emit(`rpc:${call}`, callResponse);
    });

    this.remotes[remoteID] = r;
    this.options?.onClientConnect?.(remoteID);

    requestResponseReader.on("close", () => {
      delete this.remotes[remoteID];
      this.options?.onClientDisconnect?.(remoteID);
    });
  };

  /**
   * Expose local functions and link remote ones to a WebSocket
   * @param socket Socket to link functions to
   * @param stringify Function to marshal a message
   * @param parse Function to unmarshal a message
   * @param stringifyNested Function to marshal a nested message
   * @param parseNested Function to unmarshal a nested message
   * @returns Remote functions
   */
  linkWebSocket = <T>(
    socket: WebSocket,

    stringify: (value: any) => string,
    parse: (text: string) => any,

    stringifyNested: (value: any) => T,
    parseNested: (text: T) => any
  ) => {
    const requestResponseReceiver = new WebSocketRequestResponseReader<T>(
      socket,
      parse
    );

    this.linkMessage(
      async (text: T) =>
        socket.send(marshalMessage<T>(text, undefined, stringify)),
      async (text: T) =>
        socket.send(marshalMessage<T>(undefined, text, stringify)),

      requestResponseReceiver,

      stringifyNested,
      parseNested
    );
  };

  /**
   * Expose local functions and link remote ones to a TCPSocket
   * @param socket Socket to link functions to
   * @param stringify Function to marshal a message
   * @param parse Function to unmarshal a message
   * @param stringifyNested Function to marshal a nested message
   * @param parseNested Function to unmarshal a nested message
   * @returns Remote functions
   */
  linkTCPSocket = <T>(
    socket: Socket,

    stringify: (value: any) => string,
    parse: (text: string) => any,

    stringifyNested: (value: any) => T,
    parseNested: (text: T) => any
  ) => {
    const requestResponseReceiver = new TCPSocketRequestResponseReader<T>(
      socket,
      parse
    );

    this.linkMessage(
      async (text: T) =>
        socket.write(marshalMessage<T>(text, undefined, stringify)),
      async (text: T) =>
        socket.write(marshalMessage<T>(undefined, text, stringify)),

      requestResponseReceiver,

      stringifyNested,
      parseNested
    );
  };

  forRemotes = async (cb: (remoteID: string, remote: R) => Promise<void>) => {
    // eslint-disable-next-line no-restricted-syntax
    for (const remoteID of Object.keys(this.remotes)) {
      // eslint-disable-next-line no-await-in-loop
      await cb(remoteID, this.remotes[remoteID]);
    }
  };
}
