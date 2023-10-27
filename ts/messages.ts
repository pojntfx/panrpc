interface IMessage {
  request?: string;
  response?: string;
}

export const marshalMessage = (
  request: string | undefined,
  response: string | undefined
): string => {
  const msg: IMessage = { request, response };

  return JSON.stringify(msg);
};

export const unmarshalMessage = (msg: string): IMessage => JSON.parse(msg);

interface IRequest {
  call: string;
  function: string;
  args: string[];
}

export const marshalRequest = (
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

export const unmarshalRequest = (
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

export const marshalResponse = (
  call: string,
  value: any,
  err: string
): string => {
  const res: IResponse = {
    call,
    value: btoa(JSON.stringify(value)),
    err,
  };

  return btoa(JSON.stringify(res));
};

export const unmarshalResponse = (
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
