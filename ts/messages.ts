interface IMessage<T> {
  request?: T;
  response?: T;
}

export const marshalMessage = <T>(
  request: T | undefined,
  response: T | undefined,

  stringify: (value: any) => string
): string => {
  const msg: IMessage<T> = { request, response };

  return stringify(msg);
};

export const unmarshalMessage = <T>(
  msg: string,

  parse: (text: string) => any
): IMessage<T> => parse(msg);

interface IRequest<T> {
  call: string;
  function: string;
  args: T[];
}

export const marshalRequest = <T>(
  call: string,
  functionName: string,
  args: any[],

  stringifyNested: (value: any) => T
): T => {
  const req: IRequest<T> = {
    call,
    function: functionName,
    args: args.map((arg) => stringifyNested(arg)),
  };

  return stringifyNested(req);
};

export const unmarshalRequest = <T>(
  request: T,

  parseNested: (text: T) => any
): {
  call: string;
  functionName: string;
  args: any[];
} => {
  const req: IRequest<T> = parseNested(request);

  return {
    call: req.call,
    functionName: req.function,
    args: req.args.map((arg) => parseNested(arg)),
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

  stringifyNested: (value: any) => T
): T => {
  const res: IResponse<T> = {
    call,
    value: stringifyNested(value),
    err,
  };

  return stringifyNested(res);
};

export const unmarshalResponse = <T>(
  response: T,

  parseNested: (text: T) => any
): {
  call: string;
  value: any;
  err: string;
} => {
  const res: IResponse<T> = parseNested(response);

  return {
    call: res.call,
    value: parseNested(res.value),
    err: res.err,
  };
};
