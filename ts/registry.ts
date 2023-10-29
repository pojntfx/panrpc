import { EventEmitter } from "events";
import { v4 } from "uuid";
import {
  marshalMessage,
  marshalRequest,
  marshalResponse,
  unmarshalMessage,
  unmarshalRequest,
  unmarshalResponse,
} from "./messages";

interface ICallResponse {
  value?: any;
  err: string;
}

export const ErrorCallTimedOut = "call timed out";

/**
 * Expose local functions and link remote ones to a WebSocket
 * @param socket WebSocket to use
 * @param local Local functions to expose
 * @returns Remote functions
 */
export const linkWebSocket = <R, T>(
  socket: WebSocket,

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
            err: ErrorCallTimedOut,
          };

          broker.emit(`rpc:${id}`, callResponse);
        }, timeout);

        broker.addListener(`rpc:${id}`, (e) => {
          clearTimeout(t);

          handleReturn(e);
        });

        socket.send(
          marshalMessage<T>(
            marshalRequest<T>(id, functionName, args, stringifyNested),
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
