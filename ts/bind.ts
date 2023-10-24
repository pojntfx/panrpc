import { v4 } from "uuid";
import { EventEmitter } from "events";

export interface IBindConfig {
  onOpen?: () => void;
  onError?: (e: Event) => void;
  onClose?: () => void;
  reconnectDelay?: number;
}

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

/**
 * Expose local functions and bind remote ones
 * @param getSocket Function that returns the WebSocket to use
 * @param local Local functions to expose
 * @param remote Object with expected remote function signatures
 * @param setRemote Function to set the generated remote functions with
 * @param config Additional configuration
 */
export const bind = (
  getSocket: () => WebSocket,

  local: any,

  remote: any,
  setRemote: (remote: any) => void,

  config?: IBindConfig
) => {
  const broker = new EventEmitter();

  const socket = getSocket();

  socket.addEventListener("open", () => {
    config?.onOpen?.();
  });

  socket.addEventListener("error", (e) => {
    config?.onError?.(e);
  });

  socket.addEventListener("close", async () => {
    config?.onClose?.();

    await new Promise((res) => {
      setTimeout(res, config?.reconnectDelay || 1000);
    });

    bind(getSocket, local, remote, setRemote);
  });

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

        broker.addListener(`rpc:${id}`, handleReturn);

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
  setRemote(r);

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
};
