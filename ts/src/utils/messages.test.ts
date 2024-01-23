import { describe, expect, test } from "bun:test";
import {
  marshalRequest,
  marshalResponse,
  unmarshalRequest,
  unmarshalResponse,
} from "./messages";

describe("Nested encoding", () => {
  const marshal = (value: any) => btoa(JSON.stringify(value));
  const unmarshal = (text: string) => JSON.parse(atob(text));

  describe("Request", () => {
    test("can marshal a simple request", () => {
      expect(
        marshalRequest("1", "Println", ["Hello, world!"], (v) => marshal(v))
      ).toMatchSnapshot();
    });

    test("can unmarshal a simple request", () => {
      expect(
        unmarshalRequest(
          "eyJjYWxsIjoiMSIsImZ1bmN0aW9uIjoiUHJpbnRsbiIsImFyZ3MiOlsiSWtobGJHeHZMQ0IzYjNKc1pDRWkiXX0=",
          (v) => unmarshal(v)
        )
      ).toMatchSnapshot();
    });

    test("can marshal a complex request", () => {
      expect(
        marshalRequest("1-2-3", "Add", [1, 2, { asdf: 1 }], (v) => marshal(v))
      ).toMatchSnapshot();
    });

    test("can unmarshal a complex request", () => {
      expect(
        unmarshalRequest(
          "eyJjYWxsIjoiMS0yLTMiLCJmdW5jdGlvbiI6IkFkZCIsImFyZ3MiOlsiTVE9PSIsIk1nPT0iLCJleUpoYzJSbUlqb3hmUT09Il19",
          (v) => unmarshal(v)
        )
      ).toMatchSnapshot();
    });
  });

  describe("Response", () => {
    test("can marshal a simple response with no error", () => {
      expect(
        marshalResponse("1", true, "", (v) => marshal(v))
      ).toMatchSnapshot();
    });

    test("can unmarshal a simple response with no error", () => {
      expect(
        unmarshalResponse(
          "eyJjYWxsIjoiMSIsInZhbHVlIjoiZEhKMVpRPT0iLCJlcnIiOiIifQ==",
          (v) => unmarshal(v)
        )
      ).toMatchSnapshot();
    });

    test("can marshal a simple response with error", () => {
      expect(
        marshalResponse("1", true, "test error", (v) => marshal(v))
      ).toMatchSnapshot();
    });

    test("can unmarshal a simple response with error", () => {
      expect(
        unmarshalResponse(
          "eyJjYWxsIjoiMSIsInZhbHVlIjoiZEhKMVpRPT0iLCJlcnIiOiJ0ZXN0IGVycm9yIn0=",
          (v) => unmarshal(v)
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
          (v) => marshal(v)
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a complex response with no error", () => {
      expect(
        unmarshalResponse(
          "eyJjYWxsIjoiMSIsInZhbHVlIjoiZXlKaElqb3hMQ0ppSWpwN0ltTWlPaUowWlhOMEluMTkiLCJlcnIiOiIifQ==",
          (v) => unmarshal(v)
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
          (v) => marshal(v)
        )
      ).toMatchSnapshot();
    });

    test("can unmarshal a complex response with error", () => {
      expect(
        unmarshalResponse(
          "eyJjYWxsIjoiMSIsInZhbHVlIjoiZXlKaElqb3hMQ0ppSWpwN0ltTWlPaUowWlhOMEluMTkiLCJlcnIiOiJ0ZXN0IGVycm9yIn0=",
          (v) => unmarshal(v)
        )
      ).toMatchSnapshot();
    });
  });
});

describe("Flat encoding", () => {
  const marshal = (value: any) => value;
  const unmarshal = (text: any) => text;

  describe("Request", () => {
    test("can marshal a simple request", () => {
      expect(
        marshalRequest("1", "Println", ["Hello, world!"], (v) => marshal(v))
      ).toMatchSnapshot();
    });

    test("can unmarshal a simple request", () => {
      expect(
        unmarshalRequest(
          {
            args: ["Hello, world!"],
            function: "Println",
          },
          (v) => unmarshal(v)
        )
      ).toMatchSnapshot();
    });

    test("can marshal a complex request", () => {
      expect(
        marshalRequest("1-2-3", "Add", [1, 2, { asdf: 1 }], (v) => marshal(v))
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
          (v) => unmarshal(v)
        )
      ).toMatchSnapshot();
    });
  });

  describe("Response", () => {
    test("can marshal a simple response with no error", () => {
      expect(
        marshalResponse("1", true, "", (v) => marshal(v))
      ).toMatchSnapshot();
    });

    test("can unmarshal a simple response with no error", () => {
      expect(
        unmarshalResponse(
          {
            err: "",
            value: true,
          },
          (v) => unmarshal(v)
        )
      ).toMatchSnapshot();
    });

    test("can marshal a simple response with error", () => {
      expect(
        marshalResponse("1", true, "test error", (v) => marshal(v))
      ).toMatchSnapshot();
    });

    test("can unmarshal a simple response with error", () => {
      expect(
        unmarshalResponse(
          {
            err: "test error",
            value: true,
          },
          (v) => unmarshal(v)
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
          (v) => marshal(v)
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
          (v) => unmarshal(v)
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
          (v) => marshal(v)
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
          (v) => unmarshal(v)
        )
      ).toMatchSnapshot();
    });
  });
});
