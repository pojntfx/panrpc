import { ILocalContext } from "./registry";

export class ClosureManager {
  CallClosure = async (
    ctx: ILocalContext,
    closureID: string,
    args: any[]
  ): Promise<any> => {
    console.log(
      "Calling closure with ID",
      closureID,
      "and arguments",
      args,
      "for remote with ID",
      ctx.remoteID
    );
  };
}
