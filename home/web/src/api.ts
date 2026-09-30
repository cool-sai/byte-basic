import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { HomeService } from "./gen/home/v1/home_pb";

export const client = createClient(
  HomeService,
  createConnectTransport({
    baseUrl: "/api",
  }),
);
