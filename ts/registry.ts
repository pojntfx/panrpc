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
  value: any;
  err: string;
}

export const ErrorCallTimedOut = "call timed out";

/**
 * Expose local functions and link remote ones to a WebSocket
 * @param socket WebSocket to use
 * @param local Local functions to expose
 * @returns Remote functions
 */
export const linkWebSocket = <R>(
  socket: WebSocket,

  local: any,
  remote: R,

  timeout: number
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
            value: "",
            err: ErrorCallTimedOut,
          };

          broker.emit(`rpc:${id}`, callResponse);
        }, timeout);

        broker.addListener(`rpc:${id}`, (e) => {
          clearTimeout(t);

          handleReturn(e);
        });

        socket.send(
          marshalMessage(marshalRequest(id, functionName, args), undefined)
        );
      });
  }

  socket.addEventListener("message", async (event) => {
    const msg = unmarshalMessage(event.data as string);

    if (msg.request) {
      const { call, functionName, args } = unmarshalRequest(msg.request);

      let res = "";
      try {
        const rv = await (local as any)[functionName](...args);

        res = marshalResponse(call, rv, "");
      } catch (e) {
        res = marshalResponse(call, undefined, (e as Error).message);
      }

      socket.send(marshalMessage(undefined, res));
    } else if (msg.response) {
      const { call, value, err } = unmarshalResponse(msg.response);

      const callResponse: ICallResponse = {
        value,
        err,
      };

      broker.emit(`rpc:${call}`, callResponse);
    }
  });

  return r;
};
