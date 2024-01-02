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
}

class WebSocketRequestResponseReader<T> implements IRequestResponseReader<T> {
  private requestListener?: (message: T) => void;

  private responseListener?: (message: T) => void;

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
  }

  on = (
    event: "request" | "response",
    listener: (message: T) => void
  ): this => {
    if (event === "request") {
      this.requestListener = listener;
    } else if (event === "response") {
      this.responseListener = listener;
    }

    return this;
  };
}

class TCPSocketRequestResponseReader<T> implements IRequestResponseReader<T> {
  private requestListener?: (message: T) => void;

  private responseListener?: (message: T) => void;

  constructor(socket: Socket, parse: (text: string) => any) {
    socket.addListener("data", (data: any) => {
      const msg = unmarshalMessage<T>(data.toString() as string, parse);

      if (msg.request) {
        this.requestListener?.(msg.request);
      } else if (msg.response) {
        this.responseListener?.(msg.response);
      }
    });
  }

  on = (
    event: "request" | "response",
    listener: (message: T) => void
  ): this => {
    if (event === "request") {
      this.requestListener = listener;
    } else if (event === "response") {
      this.responseListener = listener;
    }

    return this;
  };
}

/**
 * Expose local functions and link remote ones to a message-based transport
 * @param local Local functions to explose
 * @param remote Remote functions to implement
 * @param writeRequest Function to write a request
 * @param writeResponse Function to write a response
 * @param requestResponseReader Emitter to read requests and responses
 * @param stringify Function to marshal a message
 * @param parse Function to unmarshal a message
 * @returns Remote functions
 */
export const linkMessage = <L extends ILocal, R extends IRemote, T>(
  local: L,
  remote: R,

  writeRequest: (text: T) => Promise<any>,
  writeResponse: (text: T) => Promise<any>,

  requestResponseReader: IRequestResponseReader<T>,

  stringify: (value: any) => T,
  parse: (text: T) => any
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

        writeRequest(
          marshalRequest<T>(id, functionName, rest, stringify)
        ).catch(rej);
      });
  }

  requestResponseReader.on("request", async (message) => {
    const { call, functionName, args } = unmarshalRequest<T>(message, parse);

    let res: T;
    try {
      const ctx: ILocalContext = { remoteID: "1" }; // TODO: Use remote-unique ID here

      const rv = await (local as any)[functionName](ctx, ...args);

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

  return r;
};

/**
 * Expose local functions and link remote ones to a WebSocket
 * @param socket Socket to link functions to
 * @param local Local functions to expose
 * @param remote Remote functions to implement
 * @param stringify Function to marshal a message
 * @param parse Function to unmarshal a message
 * @param stringifyNested Function to marshal a nested message
 * @param parseNested Function to unmarshal a nested message
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
  const requestResponseReceiver = new WebSocketRequestResponseReader<T>(
    socket,
    parse
  );

  return linkMessage(
      local,
      remote,

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
 * @param local Local functions to expose
 * @param remote Remote functions to implement
 * @param stringify Function to marshal a message
 * @param parse Function to unmarshal a message
 * @param stringifyNested Function to marshal a nested message
 * @param parseNested Function to unmarshal a nested message
 * @returns Remote functions
 */
export const linkTCPSocket = <L extends ILocal, R extends IRemote, T>(
  socket: Socket,

  local: L,
  remote: R,

  stringify: (value: any) => string,
  parse: (text: string) => any,

  stringifyNested: (value: any) => T,
  parseNested: (text: T) => any
) => {
  const requestResponseReceiver = new TCPSocketRequestResponseReader<T>(
    socket,
    parse
  );

  return linkMessage(
      local,
      remote,

      async (text: T) =>
        socket.write(marshalMessage<T>(text, undefined, stringify)),
      async (text: T) =>
        socket.write(marshalMessage<T>(undefined, text, stringify)),

      requestResponseReceiver,

      stringifyNested,
      parseNested
  );
};
