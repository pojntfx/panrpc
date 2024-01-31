import { EventEmitter } from "events";
import "reflect-metadata";
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

    writeRequest: WritableStreamDefaultWriter<T>["write"],

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

      writeRequest(marshalRequest<T>(callID, name, args, marshal)).catch(rej);
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
    writeRequest: WritableStreamDefaultWriter<T>["write"],
    writeResponse: WritableStreamDefaultWriter<T>["write"],

    readRequest: ReadableStreamDefaultReader<T>["read"],
    readResponse: ReadableStreamDefaultReader<T>["read"],

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

        writeRequest,

        marshal,

        this.closureManager
      );
    }

    let closed = false;

    const remoteID = v4();

    const processRequest = async ({
      done,
      value: message,
    }: {
      done: boolean;
      value?: T;
    }) => {
      if (done) {
        if (!closed) {
          closed = true;

          delete this.remotes[remoteID];
          this.options?.onClientDisconnect?.(remoteID);
        }

        return;
      }

      if (!message) {
        readRequest().then(processRequest);

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

                      writeRequest,

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

        await writeResponse(resp);
      } finally {
        readRequest().then(processRequest);
      }
    };
    readRequest().then(processRequest);

    const processResponse = async ({
      done,
      value: message,
    }: {
      done: boolean;
      value?: T;
    }) => {
      if (done) {
        if (!closed) {
          closed = true;

          delete this.remotes[remoteID];
          this.options?.onClientDisconnect?.(remoteID);
        }

        return;
      }

      if (!message) {
        readResponse().then(processResponse);

        return;
      }

      try {
        const { call, value, err } = unmarshalResponse<T>(message, unmarshal);

        const callResponse: ICallResponse = {
          value,
          err,
        };

        responseResolver.emit(`rpc:${call}`, callResponse);
      } finally {
        readResponse().then(processResponse);
      }
    };
    readResponse().then(processResponse);

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

    const [requestDecoder, responseDecoder] = decoder.tee();
    const [requestDecoderReader, responseDecoderReader] = [
      requestDecoder.getReader(),
      responseDecoder.getReader(),
    ];

    const requestReader = new ReadableStream({
      start(controller) {
        requestDecoderReader
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

            if (message?.request) {
              controller.enqueue(unmarshal(message?.request));
            }

            requestDecoderReader.read().then(process);
          });
      },
    });

    const responseReader = new ReadableStream({
      start(controller) {
        responseDecoderReader
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

            if (message?.response) {
              controller.enqueue(unmarshal(message?.response));
            }

            responseDecoderReader.read().then(process);
          });
      },
    });

    this.linkMessage(
      requestWriter.getWriter().write,
      responseWriter.getWriter().write,

      requestReader.getReader().read,
      responseReader.getReader().read,

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
