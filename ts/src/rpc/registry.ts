import "reflect-metadata";
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
    responseResolver: EventTarget,

    requestWriter: WritableStreamDefaultWriter<T>,

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

      const callID = crypto.randomUUID();

      const abortListener = () => {
        ctx?.signal?.removeEventListener("abort", abortListener);

        closureFreers.map((free) => free());

        const callResponse: ICallResponse = {
          err: ErrorCallCancelled,
        };

        responseResolver.dispatchEvent(
          new CustomEvent(`rpc:${callID}`, { detail: callResponse })
        );
      };
      ctx?.signal?.addEventListener("abort", abortListener);

      const returnListener = (event: Event) => {
        const { value, err } = (event as CustomEvent<ICallResponse>).detail;

        responseResolver.removeEventListener(`rpc:${callID}`, returnListener);

        closureFreers.map((free) => free());

        if (err) {
          rej(new Error(err));
        } else {
          res(value);
        }
      };
      responseResolver.addEventListener(`rpc:${callID}`, returnListener);

      requestWriter
        .write(marshalRequest<T>(callID, name, args, marshal))
        .catch(rej);
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
    requestWriter: WritableStreamDefaultWriter<T>,
    responseWriter: WritableStreamDefaultWriter<T>,

    requestReader: ReadableStreamDefaultReader<T>,
    responseReader: ReadableStreamDefaultReader<T>,

    marshal: (value: any) => T,
    unmarshal: (text: T) => any
  ) => {
    const responseResolver = new EventTarget();

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

    let closed = false;

    const remoteID = crypto.randomUUID();

    const that = this;

    requestReader
      .read()
      .then(async function process({
        done,
        value: message,
      }: {
        done: boolean;
        value?: T;
      }) {
        if (done) {
          if (!closed) {
            closed = true;

            delete that.remotes[remoteID];
            that.options?.onClientDisconnect?.(remoteID);
          }

          return;
        }

        if (!message) {
          requestReader.read().then(process);

          return;
        }

        try {
          const { call, functionName, args } = unmarshalRequest<T>(
            message,
            unmarshal
          );

          let resp: T;
          try {
            if (functionName === constructorFunctionName) {
              throw new Error(ErrorCannotCallNonFunction);
            }

            let fn = (that.local as any)[functionName];
            if (typeof fn !== "function") {
              fn = (that.closureManager as any)[functionName];

              if (typeof fn !== "function") {
                throw new Error(ErrorCannotCallNonFunction);
              }
            }

            const remoteClosureParameterIndexes: number[] | undefined =
              Reflect.getMetadata(remoteClosureKey, that.local, functionName);

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

                        that.closureManager
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

          await responseWriter.write(resp);
        } finally {
          requestReader.read().then(process);
        }
      });

    responseReader
      .read()
      .then(async function process({
        done,
        value: message,
      }: {
        done: boolean;
        value?: T;
      }) {
        if (done) {
          if (!closed) {
            closed = true;

            delete that.remotes[remoteID];
            that.options?.onClientDisconnect?.(remoteID);
          }

          return;
        }

        if (!message) {
          responseReader.read().then(process);

          return;
        }

        try {
          const { call, value, err } = unmarshalResponse<T>(message, unmarshal);

          const callResponse: ICallResponse = {
            value,
            err,
          };

          responseResolver.dispatchEvent(
            new CustomEvent(`rpc:${call}`, { detail: callResponse })
          );
        } finally {
          responseReader.read().then(process);
        }
      });

    this.remotes[remoteID] = r;

    this.options?.onClientConnect?.(remoteID);
  };

  linkStream = <T>(
    encoder: WritableStream,
    decoder: ReadableStream,

    marshal: (value: any) => T,
    unmarshal: (text: T) => any
  ) => {
    const encoderWriter = encoder.getWriter();

    const requestWriter = new WritableStream({
      write(chunk: T) {
        const msg: IMessage<T> = { request: chunk };

        return encoderWriter.write(msg);
      },
    });

    const responseWriter = new WritableStream({
      write(chunk: T) {
        const msg: IMessage<T> = { response: chunk };

        return encoderWriter.write(msg);
      },
    });

    const [messageDecoderForRequests, messageDecoderForResponses] =
      decoder.tee();
    const [messageDecoderForRequestsReader, messageDecoderForResponsesReader] =
      [
        messageDecoderForRequests.getReader(),
        messageDecoderForResponses.getReader(),
      ];

    const requestReader = new ReadableStream({
      start(controller) {
        messageDecoderForRequestsReader
          .read()
          .then(function process({
            done,
            value: message,
          }: {
            done: boolean;
            value?: IMessage<T>;
          }) {
            if (done) {
              controller.close();

              return;
            }

            if (!message?.request) {
              messageDecoderForRequestsReader.read().then(process);

              return;
            }

            controller.enqueue(unmarshal(message?.request));

            messageDecoderForRequestsReader.read().then(process);
          });
      },
    });

    const responseReader = new ReadableStream({
      start(controller) {
        messageDecoderForResponsesReader
          .read()
          .then(function process({
            done,
            value: message,
          }: {
            done: boolean;
            value?: IMessage<T>;
          }) {
            if (done) {
              controller.close();

              return;
            }

            if (!message?.response) {
              messageDecoderForResponsesReader.read().then(process);

              return;
            }

            controller.enqueue(unmarshal(message?.response));

            messageDecoderForResponsesReader.read().then(process);
          });
      },
    });

    this.linkMessage(
      requestWriter.getWriter(),
      responseWriter.getWriter(),

      requestReader.getReader(),
      responseReader.getReader(),

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
