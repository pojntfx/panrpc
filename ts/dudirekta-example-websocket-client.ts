/* eslint-disable no-alert */
/* eslint-disable no-console */
import { env } from "process";
import { bind } from "./bind";

let remote = {
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  Increment: async (delta: number): Promise<number> => 0,
};

const raddr = env.RADDR || "ws://127.0.0.1:1337";

await new Promise<void>((res, rej) => {
  bind(
    () => new WebSocket(raddr),
    {
      Println: async (msg: string) => {
        console.log(msg);
      },
    },
    remote,
    (r) => {
      remote = r;
    },
    {
      onOpen: res,
      onError: rej,
    }
  );
});

console.log("Connected to", raddr);

// eslint-disable-next-line no-constant-condition
while (true) {
  const line =
    prompt(`Enter one of the following letters followed by <ENTER> to run a function on the remote(s):

- a: Increment remote counter by one
- b: Decrement remote counter by one
`);

  console.log(line, remote);
}
