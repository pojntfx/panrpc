import { describe, expect, test } from "bun:test";
import {
  marshalMessage,
  marshalRequest,
  marshalResponse,
  unmarshalMessage,
  unmarshalRequest,
  unmarshalResponse,
} from "./messages";

describe("Nested encoding", () => {
  const stringify = JSON.stringify;
  const parse = JSON.parse;

  const stringifyNested = (value: any) => btoa(JSON.stringify(value));
  const parseNested = (text: string) => JSON.parse(atob(text));

  describe("Message", () => {
    test("can marshal a message with a request set", () => {
      expect(
        marshalMessage(
          marshalRequest("1", "Println", ["Hello, world!"], (v) =>
            stringifyNested(v)
          ),
          "",
          stringify
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a message with a request set", () => {
      expect(
        unmarshalMessage(
          `{"request":"eyJjYWxsIjoiMSIsImZ1bmN0aW9uIjoiUHJpbnRsbiIsImFyZ3MiOlsiSWtobGJHeHZMQ0IzYjNKc1pDRWkiXX0=","response":""}`,
          parse
        )
      ).toMatchSnapshot();
    });

    test("can marshal a message with a response set", () => {
      expect(
        marshalMessage(
          "",
          marshalResponse("1", true, "", (v) => stringifyNested(v)),
          stringify
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a message with a response set", () => {
      expect(
        unmarshalMessage(
          `{"request":"","response":"eyJjYWxsIjoiMSIsInZhbHVlIjoiZEhKMVpRPT0iLCJlcnIiOiIifQ=="}`,
          parse
        )
      ).toMatchSnapshot();
    });
  });

  describe("Request", () => {
    test("can marshal a simple request", () => {
      expect(
        marshalRequest("1", "Println", ["Hello, world!"], (v) =>
          stringifyNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a simple request", () => {
      expect(
        unmarshalRequest(
          "eyJjYWxsIjoiMSIsImZ1bmN0aW9uIjoiUHJpbnRsbiIsImFyZ3MiOlsiSWtobGJHeHZMQ0IzYjNKc1pDRWkiXX0=",
          (v) => parseNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can marshal a complex request", () => {
      expect(
        marshalRequest("1-2-3", "Add", [1, 2, { asdf: 1 }], (v) =>
          stringifyNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a complex request", () => {
      expect(
        unmarshalRequest(
          "eyJjYWxsIjoiMS0yLTMiLCJmdW5jdGlvbiI6IkFkZCIsImFyZ3MiOlsiTVE9PSIsIk1nPT0iLCJleUpoYzJSbUlqb3hmUT09Il19",
          (v) => parseNested(v)
        )
      ).toMatchSnapshot();
    });
  });

  describe("Response", () => {
    test("can marshal a simple response with no error", () => {
      expect(
        marshalResponse("1", true, "", (v) => stringifyNested(v))
      ).toMatchSnapshot();
    });

    test("can unmarshal a simple response with no error", () => {
      expect(
        unmarshalResponse(
          "eyJjYWxsIjoiMSIsInZhbHVlIjoiZEhKMVpRPT0iLCJlcnIiOiIifQ==",
          (v) => parseNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can marshal a simple response with error", () => {
      expect(
        marshalResponse("1", true, "test error", (v) => stringifyNested(v))
      ).toMatchSnapshot();
    });

    test("can unmarshal a simple response with error", () => {
      expect(
        unmarshalResponse(
          "eyJjYWxsIjoiMSIsInZhbHVlIjoiZEhKMVpRPT0iLCJlcnIiOiJ0ZXN0IGVycm9yIn0=",
          (v) => parseNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can marshal a complex response with no error", () => {
      expect(
        marshalResponse(
          "1",
          {
            a: 1,
            b: {
              c: "test",
            },
          },
          "",
          (v) => stringifyNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a complex response with no error", () => {
      expect(
        unmarshalResponse(
          "eyJjYWxsIjoiMSIsInZhbHVlIjoiZXlKaElqb3hMQ0ppSWpwN0ltTWlPaUowWlhOMEluMTkiLCJlcnIiOiIifQ==",
          (v) => parseNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can marshal a complex response with error", () => {
      expect(
        marshalResponse(
          "1",
          {
            a: 1,
            b: {
              c: "test",
            },
          },
          "test error",
          (v) => stringifyNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a complex response with error", () => {
      expect(
        unmarshalResponse(
          "eyJjYWxsIjoiMSIsInZhbHVlIjoiZXlKaElqb3hMQ0ppSWpwN0ltTWlPaUowWlhOMEluMTkiLCJlcnIiOiJ0ZXN0IGVycm9yIn0=",
          (v) => parseNested(v)
        )
      ).toMatchSnapshot();
    });
  });
});

describe("Flat encoding", () => {
  const stringify = JSON.stringify;
  const parse = JSON.parse;

  const stringifyNested = (value: any) => value;
  const parseNested = (text: any) => text;

  describe("Message", () => {
    test("can marshal a message with a request set", () => {
      expect(
        marshalMessage(
          marshalRequest("1", "Println", ["Hello, world!"], (v) =>
            stringifyNested(v)
          ),
          "",
          stringify
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a message with a request set", () => {
      expect(
        unmarshalMessage(
          `{"request":{"call":"1","function":"Println","args":["Hello, world!"]},"response":""}`,
          parse
        )
      ).toMatchSnapshot();
    });

    test("can marshal a message with a response set", () => {
      expect(
        marshalMessage(
          "",
          marshalResponse("1", true, "", (v) => stringifyNested(v)),
          stringify
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a message with a response set", () => {
      expect(
        unmarshalMessage(
          `{"request":"","response":{"call":"1","value":true,"err":""}}`,
          parse
        )
      ).toMatchSnapshot();
    });
  });

  describe("Request", () => {
    test("can marshal a simple request", () => {
      expect(
        marshalRequest("1", "Println", ["Hello, world!"], (v) =>
          stringifyNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a simple request", () => {
      expect(
        unmarshalRequest(
          {
            args: ["Hello, world!"],
            function: "Println",
          },
          (v) => parseNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can marshal a complex request", () => {
      expect(
        marshalRequest("1-2-3", "Add", [1, 2, { asdf: 1 }], (v) =>
          stringifyNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a complex request", () => {
      expect(
        unmarshalRequest(
          {
            args: [
              1,
              2,
              {
                asdf: 1,
              },
            ],
            function: "Add",
          },
          (v) => parseNested(v)
        )
      ).toMatchSnapshot();
    });
  });

  describe("Response", () => {
    test("can marshal a simple response with no error", () => {
      expect(
        marshalResponse("1", true, "", (v) => stringifyNested(v))
      ).toMatchSnapshot();
    });

    test("can unmarshal a simple response with no error", () => {
      expect(
        unmarshalResponse(
          {
            err: "",
            value: true,
          },
          (v) => parseNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can marshal a simple response with error", () => {
      expect(
        marshalResponse("1", true, "test error", (v) => stringifyNested(v))
      ).toMatchSnapshot();
    });

    test("can unmarshal a simple response with error", () => {
      expect(
        unmarshalResponse(
          {
            err: "test error",
            value: true,
          },
          (v) => parseNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can marshal a complex response with no error", () => {
      expect(
        marshalResponse(
          "1",
          {
            a: 1,
            b: {
              c: "test",
            },
          },
          "",
          (v) => stringifyNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a complex response with no error", () => {
      expect(
        unmarshalResponse(
          {
            err: "",
            value: {
              a: 1,
              b: {
                c: "test",
              },
            },
          },
          (v) => parseNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can marshal a complex response with error", () => {
      expect(
        marshalResponse(
          "1",
          {
            a: 1,
            b: {
              c: "test",
            },
          },
          "test error",
          (v) => stringifyNested(v)
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a complex response with error", () => {
      expect(
        unmarshalResponse(
          {
            err: "test error",
            value: {
              a: 1,
              b: {
                c: "test",
              },
            },
          },
          (v) => parseNested(v)
        )
      ).toMatchSnapshot();
    });
  });
});
