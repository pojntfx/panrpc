import { EventEmitter } from "events";
import "reflect-metadata";
import { Readable, Writable } from "stream";
import Chain from "stream-chain";
import { v4 } from "uuid";
import {
  IMessage,
  marshalRequest,
  marshalResponse,
  unmarshalRequest,
  unmarshalResponse,
} from "../utils/messages";
import { ILocalContext, IRemoteContext } from "./context";
import { ClosureManager, registerClosure } from "./manager";

export const ErrorCallCancelled = "call timed out";
export const ErrorCannotCallNonFunction = "can not call non function";

const constructorFunctionName = "constructor";

interface ICallResponse {
  value?: any;
  err: string;
}

export interface IOptions {
  onClientConnect?: (remoteID: string) => void;
  onClientDisconnect?: (remoteID: string) => void;
}

const remoteClosureKey = Symbol("required");
export const remoteClosure = (
  target: Object,
  propertyKey: string | symbol,
  parameterIndex: number
) => {
  const remoteClosureParameterIndexes: number[] =
    Reflect.getOwnMetadata(remoteClosureKey, target, propertyKey) || [];
  remoteClosureParameterIndexes.push(parameterIndex);

  Reflect.defineMetadata(
    remoteClosureKey,
    remoteClosureParameterIndexes,
    target,
    propertyKey
  );
};

const makeRPC =
  <T>(
    name: string,
    responseResolver: EventEmitter,

    requestWriter: Writable,

    marshal: (value: any) => T,

    closureManager: ClosureManager
  ) =>
  async (ctx: IRemoteContext, ...rest: any[]) =>
    new Promise((res, rej) => {
      if (ctx?.signal?.aborted) {
        rej(new Error(ErrorCallCancelled));

        return;
      }

      const closureFreers: (() => void)[] = [];
      const args = rest.map((arg) => {
        if (typeof arg === "function") {
          const { closureID, freeClosure } = registerClosure(
            closureManager,
            arg as Function
          );

          closureFreers.push(freeClosure);

          return closureID;
        }

        return arg;
      });

      const callID = v4();

      const abortListener = () => {
        ctx?.signal?.removeEventListener("abort", abortListener);

        closureFreers.map((free) => free());

        const callResponse: ICallResponse = {
          err: ErrorCallCancelled,
        };

        responseResolver.emit(`rpc:${callID}`, callResponse);
      };
      ctx?.signal?.addEventListener("abort", abortListener);

      const returnListener = ({ value, err }: ICallResponse) => {
        responseResolver.removeListener(`rpc:${callID}`, returnListener);

        closureFreers.map((free) => free());

        if (err) {
          rej(new Error(err));
        } else {
          res(value);
        }
      };
      responseResolver.addListener(`rpc:${callID}`, returnListener);

      requestWriter.write(
        marshalRequest<T>(callID, name, args, marshal),
        (e) => e && rej(e)
      );
    });

export class Registry<L extends Object, R extends Object> {
  private closureManager: ClosureManager;

  private remotes: {
    [remoteID: string]: R;
  } = {};

  constructor(
    private local: L,
    private remote: R,

    private options?: IOptions
  ) {
    this.closureManager = new ClosureManager();
  }

  linkMessage = <T>(
    requestWriter: Writable,
    responseWriter: Writable,

    requestReader: Readable,
    responseReader: Readable,

    marshal: (value: any) => T,
    unmarshal: (text: T) => any
  ) => {
    const responseResolver = new EventEmitter();

    const r: R = {} as R;
    // eslint-disable-next-line no-restricted-syntax
    for (const functionName of Object.getOwnPropertyNames(
      Object.getPrototypeOf(this.remote)
    )) {
      if (functionName === constructorFunctionName) {
        // eslint-disable-next-line no-continue
        continue;
      }

      (r as any)[functionName] = makeRPC(
        functionName,
        responseResolver,

        requestWriter,

        marshal,

        this.closureManager
      );
    }

    const remoteID = v4();

    requestReader.pipe(
      new Chain([
        async (message: T) => {
          const { call, functionName, args } = unmarshalRequest<T>(
            message,
            unmarshal
          );

          let resp: T;
          try {
            if (functionName === constructorFunctionName) {
              throw new Error(ErrorCannotCallNonFunction);
            }

            let fn = (this.local as any)[functionName];
            if (typeof fn !== "function") {
              fn = (this.closureManager as any)[functionName];

              if (typeof fn !== "function") {
                throw new Error(ErrorCannotCallNonFunction);
              }
            }

            const remoteClosureParameterIndexes: number[] | undefined =
              Reflect.getMetadata(remoteClosureKey, this.local, functionName);

            const ctx: ILocalContext = { remoteID };

            const rv = await fn(
              ctx,
              ...args.map((closureID, index) =>
                remoteClosureParameterIndexes?.includes(index + 1)
                  ? (closureCtx: IRemoteContext, ...closureArgs: any[]) => {
                      const rpc = makeRPC<T>(
                        "CallClosure",
                        responseResolver,

                        requestWriter,

                        marshal,

                        this.closureManager
                      );

                      return rpc(closureCtx, closureID, closureArgs);
                    }
                  : closureID
              )
            );

            resp = marshalResponse<T>(call, rv, "", marshal);
          } catch (e) {
            resp = marshalResponse<T>(
              call,
              undefined,
              (e as Error).message,
              marshal
            );
          }

          await new Promise<void>((res, rej) => {
            responseWriter.write(resp, (e) => (e ? rej(e) : res()));
          });
        },
      ])
    );

    responseReader.pipe(
      new Chain([
        async (message: T) => {
          const { call, value, err } = unmarshalResponse<T>(message, unmarshal);

          const callResponse: ICallResponse = {
            value,
            err,
          };

          responseResolver.emit(`rpc:${call}`, callResponse);
        },
      ])
    );

    this.remotes[remoteID] = r;

    let closed = false;

    requestReader.on("close", () => {
      if (!closed) {
        closed = true;

        delete this.remotes[remoteID];
        this.options?.onClientDisconnect?.(remoteID);
      }
    });

    responseReader.on("close", () => {
      if (!closed) {
        closed = true;

        delete this.remotes[remoteID];
        this.options?.onClientDisconnect?.(remoteID);
      }
    });

    this.options?.onClientConnect?.(remoteID);
  };

  linkStream = <T>(
    encoder: Writable,
    decoder: Readable,

    marshal: (value: any) => T,
    unmarshal: (text: T) => any
  ) => {
    const requestWriter = new Chain([
      (v: T) => {
        const msg: IMessage<T> = { request: v };

        return msg;
      },
    ]);
    requestWriter.pipe(encoder);

    const responseWriter = new Chain([
      (v: T) => {
        const msg: IMessage<T> = { response: v };

        return msg;
      },
    ]);
    responseWriter.pipe(encoder);

    const requestReader = new Chain([(v: T) => v]);
    const responseReader = new Chain([(v: T) => v]);

    decoder.on("close", () => {
      requestReader.destroy();
      responseReader.destroy();
    });

    decoder.pipe(
      new Chain([
        (msg: IMessage<T>) => {
          if (msg.request) {
            requestReader.write(msg.request);
          } else {
            responseReader.write(msg.response);
          }
        },
      ])
    );

    this.linkMessage(
      requestWriter,
      responseWriter,

      requestReader,
      responseReader,

      marshal,
      unmarshal
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
