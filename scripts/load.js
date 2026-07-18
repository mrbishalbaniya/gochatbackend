# k6 smoke load script example
# k6 run scripts/load.js
import http from "k6/http";
import { check, sleep } from "k6";

export const options = {
  vus: 10,
  duration: "30s",
};

const BASE = __ENV.API_URL || "http://localhost:8080";

export default function () {
  const res = http.get(`${BASE}/health`);
  check(res, { "health 200": (r) => r.status === 200 });
  sleep(1);
}
