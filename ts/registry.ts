import { v4 } from "uuid";
import { EventEmitter } from "events";

interface IMessage {
  request?: string;
  response?: string;
}

interface IRequest {
  call: string;
  function: string;
  args: string[];
}

interface IResponse {
  call: string;
  value: string;
  err: string;
}

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

        const req: IRequest = {
          call: id,
          function: functionName,
          args: args.map((arg) => btoa(JSON.stringify(arg))),
        };

        const outMsg: IMessage = {
          request: btoa(JSON.stringify(req)),
        };

        socket.send(JSON.stringify(outMsg));
      });
  }

  socket.addEventListener("message", async (event) => {
    const inMsg: IMessage = JSON.parse(event.data as string);

    if (inMsg.request) {
      const req: IRequest = JSON.parse(atob(inMsg.request));

      const args = req.args.map((arg) => JSON.parse(atob(arg)));

      const res: IResponse = {
        call: req.call,
        value: "",
        err: "",
      };

      try {
        const rv = await (local as any)[req.function](...args);

        res.value = btoa(JSON.stringify(rv));
      } catch (e) {
        res.err = (e as Error).message;
      }

      const outMsg: IMessage = {
        response: btoa(JSON.stringify(res)),
      };

      socket.send(JSON.stringify(outMsg));
    } else if (inMsg.response) {
      const res: IResponse = JSON.parse(atob(inMsg.response));

      const value = JSON.parse(atob(res.value));

      const callResponse: ICallResponse = {
        value,
        err: res.err,
      };

      broker.emit(`rpc:${res.call}`, callResponse);
    }
  });

  return r;
};
