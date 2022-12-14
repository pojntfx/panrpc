import { v4 } from "uuid";

export interface IBindConfig {
  onOpen?: () => void;
  onError?: (e: Event) => void;
  onClose?: () => void;
  reconnectDelay?: number;
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

        const handleReturn = ({ detail }: any) => {
          const [rv, err] = detail;

          if (err) {
            rej(new Error(err));
          } else {
            res(rv);
          }

          window.removeEventListener(`rpc:${id}`, handleReturn);
        };

        window.addEventListener(`rpc:${id}`, handleReturn);

        socket.send(JSON.stringify([true, id, functionName, args]));
      });
  }
  setRemote(r);

  socket.addEventListener("message", async (event) => {
    const msg = JSON.parse(event.data);

    if (msg[0]) {
      // eslint-disable-next-line @typescript-eslint/no-unused-vars, @typescript-eslint/naming-convention
      const [_, id, functionName, args] = msg;

      try {
        const res = await (local as any)[functionName](...args);

        socket.send(JSON.stringify([false, id, res, ""]));
      } catch (e) {
        socket.send(JSON.stringify([false, id, "", (e as Error).message]));
      }
    } else {
      window.dispatchEvent(
        new CustomEvent(`rpc:${msg[1]}`, {
          detail: msg.slice(2),
        })
      );
    }
  });
};
