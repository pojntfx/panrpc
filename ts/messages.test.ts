import { describe, expect, test } from "bun:test";
import {
  marshalMessage,
  marshalRequest,
  marshalResponse,
  unmarshalMessage,
  unmarshalRequest,
  unmarshalResponse,
} from "./messages";

describe("Message", () => {
  test("can marshal a message with a request set", () => {
    expect(
      marshalMessage(marshalRequest("1", "Println", ["Hello, world!"]), "")
    ).toMatchSnapshot();
  });

  test("can unmarshal a message with a request set", () => {
    expect(
      unmarshalMessage(
        `{"request":"eyJjYWxsIjoiMSIsImZ1bmN0aW9uIjoiUHJpbnRsbiIsImFyZ3MiOlsiSWtobGJHeHZMQ0IzYjNKc1pDRWkiXX0=","response":""}`
      )
    ).toMatchSnapshot();
  });

  test("can marshal a message with a response set", () => {
    expect(
      marshalMessage("", marshalResponse("1", true, ""))
    ).toMatchSnapshot();
  });

  test("can unmarshal a message with a response set", () => {
    expect(
      unmarshalMessage(
        `{"request":"","response":"eyJjYWxsIjoiMSIsInZhbHVlIjoiZEhKMVpRPT0iLCJlcnIiOiIifQ=="}`
      )
    ).toMatchSnapshot();
  });
});

describe("Request", () => {
  test("can marshal a simple request", () => {
    expect(marshalRequest("1", "Println", ["Hello, world!"])).toMatchSnapshot();
  });

  test("can unmarshal a simple request", () => {
    expect(
      unmarshalRequest(
        "eyJjYWxsIjoiMSIsImZ1bmN0aW9uIjoiUHJpbnRsbiIsImFyZ3MiOlsiSWtobGJHeHZMQ0IzYjNKc1pDRWkiXX0="
      )
    ).toMatchSnapshot();
  });

  test("can marshal a complex request", () => {
    expect(
      marshalRequest("1-2-3", "Add", [1, 2, { asdf: 1 }])
    ).toMatchSnapshot();
  });

  test("can unmarshal a complex request", () => {
    expect(
      unmarshalRequest(
        "eyJjYWxsIjoiMS0yLTMiLCJmdW5jdGlvbiI6IkFkZCIsImFyZ3MiOlsiTVE9PSIsIk1nPT0iLCJleUpoYzJSbUlqb3hmUT09Il19"
      )
    ).toMatchSnapshot();
  });
});

describe("Response", () => {
  test("can marshal a simple response with no error", () => {
    expect(marshalResponse("1", true, "")).toMatchSnapshot();
  });

  test("can unmarshal a simple response with no error", () => {
    expect(
      unmarshalResponse(
        "eyJjYWxsIjoiMSIsInZhbHVlIjoiZEhKMVpRPT0iLCJlcnIiOiIifQ=="
      )
    ).toMatchSnapshot();
  });

  test("can marshal a simple response with error", () => {
    expect(marshalResponse("1", true, "test error")).toMatchSnapshot();
  });

  test("can unmarshal a simple response with error", () => {
    expect(
      unmarshalResponse(
        "eyJjYWxsIjoiMSIsInZhbHVlIjoiZEhKMVpRPT0iLCJlcnIiOiJ0ZXN0IGVycm9yIn0="
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
        ""
      )
    ).toMatchSnapshot();
  });

  test("can unmarshal a complex response with no error", () => {
    expect(
      unmarshalResponse(
        "eyJjYWxsIjoiMSIsInZhbHVlIjoiZXlKaElqb3hMQ0ppSWpwN0ltTWlPaUowWlhOMEluMTkiLCJlcnIiOiIifQ=="
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
        "test error"
      )
    ).toMatchSnapshot();
  });

  test("can unmarshal a complex response with error", () => {
    expect(
      unmarshalResponse(
        "eyJjYWxsIjoiMSIsInZhbHVlIjoiZXlKaElqb3hMQ0ppSWpwN0ltTWlPaUowWlhOMEluMTkiLCJlcnIiOiJ0ZXN0IGVycm9yIn0="
      )
    ).toMatchSnapshot();
  });
});
