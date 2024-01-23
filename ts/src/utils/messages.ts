export interface IMessage<T> {
  request?: T;
  response?: T;
}

interface IRequest<T> {
  call: string;
  function: string;
  args: T[];
}

export const marshalRequest = <T>(
  call: string,
  functionName: string,
  args: any[],

  marshal: (value: any) => T
): T => {
  const req: IRequest<T> = {
    call,
    function: functionName,
    args: args.map((arg) => marshal(arg)),
  };

  return marshal(req);
};

export const unmarshalRequest = <T>(
  request: T,

  parse: (text: T) => any
): {
  call: string;
  functionName: string;
  args: any[];
} => {
  const req: IRequest<T> = parse(request);

  return {
    call: req.call,
    functionName: req.function,
    args: req.args.map((arg) => parse(arg)),
  };
};

interface IResponse<T> {
  call: string;
  value: T;
  err: string;
}

export const marshalResponse = <T>(
  call: string,
  value: any,
  err: string,

  marshal: (value: any) => T
): T => {
  const res: IResponse<T> = {
    call,
    value: marshal(value),
    err,
  };

  return marshal(res);
};

export const unmarshalResponse = <T>(
  response: T,

  unmarshal: (text: T) => any
): {
  call: string;
  value: any;
  err: string;
} => {
  const res: IResponse<T> = unmarshal(response);

  return {
    call: res.call,
    value: unmarshal(res.value),
    err: res.err,
  };
};
