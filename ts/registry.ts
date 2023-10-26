import { v4 } from "uuid";
import { EventEmitter } from "events";

interface IMessage {
  request?: string;
  response?: string;
}

const marshalMessage = (
  request: string | undefined,
  response: string | undefined
): string => {
  const msg: IMessage = { request, response };

  return JSON.stringify(msg);
};

const unmarshalMessage = (msg: string): IMessage => JSON.parse(msg);

interface IRequest {
  call: string;
  function: string;
  args: string[];
}

const marshalRequest = (
  call: string,
  functionName: string,
  args: any[]
): string => {
  const req: IRequest = {
    call,
    function: functionName,
    args: args.map((arg) => btoa(JSON.stringify(arg))),
  };

  return btoa(JSON.stringify(req));
};

const unmarshalRequest = (
  request: string
): {
  call: string;
  functionName: string;
  args: any[];
} => {
  const req: IRequest = JSON.parse(atob(request));

  return {
    call: req.call,
    functionName: req.function,
    args: req.args.map((arg) => JSON.parse(atob(arg))),
  };
};

interface IResponse {
  call: string;
  value: string;
  err: string;
}

const marshalResponse = (call: string, value: any, err: string): string => {
  const res: IResponse = {
    call,
    value: btoa(JSON.stringify(value)),
    err,
  };

  return btoa(JSON.stringify(res));
};

const unmarshalResponse = (
  response: string
): {
  call: string;
  value: any;
  err: string;
} => {
  const res: IResponse = JSON.parse(atob(response));

  return {
    call: res.call,
    value: JSON.parse(atob(res.value)),
    err: res.err,
  };
};

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
    const inMsg: IMessage = unmarshalMessage(event.data as string);

    if (inMsg.request) {
      const { call, functionName, args } = unmarshalRequest(inMsg.request);

      let res = "";
      try {
        const rv = await (local as any)[functionName](...args);

        res = marshalResponse(call, rv, "");
      } catch (e) {
        res = marshalResponse(call, undefined, (e as Error).message);
      }

      socket.send(marshalMessage(undefined, res));
    } else if (inMsg.response) {
      const { call, value, err } = unmarshalResponse(inMsg.response);

      const callResponse: ICallResponse = {
        value,
        err,
      };

      broker.emit(`rpc:${call}`, callResponse);
    }
  });

  return r;
};
