import { ILocalContext } from "./context";

export const ErrorClosureDoesNotExist = "closure does not exist";

export class ClosureManager {
  public closures: {
    [closureID: string]: Function;
  } = {};

  CallClosure = (
    ctx: ILocalContext,
    closureID: string,
    args: any[]
  ): Promise<any> => {
    const fn = (this.closures as any)[closureID];
    if (typeof fn !== "function") {
      throw new Error(ErrorClosureDoesNotExist);
    }

    return fn(ctx, ...args);
  };
}

export const registerClosure = (m: ClosureManager, fn: Function) => {
  const closureID = crypto.randomUUID();

  // eslint-disable-next-line no-param-reassign
  m.closures[closureID] = fn;

  return {
    closureID,
    freeClosure: () => {
      // eslint-disable-next-line no-param-reassign
      delete m.closures[closureID];
    },
  };
};
