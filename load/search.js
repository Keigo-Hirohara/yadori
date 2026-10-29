import http from 'k6/http';
import { check } from 'k6';

const baseUrl = __ENV.BASE_URL || 'http://localhost:8080/api/v1';

const rate = 70;

export const options = {
  scenarios: {
    warmup: {
      executor: 'constant-arrival-rate',
      rate: rate,
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 50,
      maxVUs: 200,
    },
    measure: {
      executor: 'constant-arrival-rate',
      rate: rate,
      timeUnit: '1s',
      duration: '3m',
      startTime: '30s',
      preAllocatedVUs: 50,
      maxVUs: 200,
    },
  },
  thresholds: {
    'http_req_duration{scenario:measure}': ['p(95)<150', 'p(99)<300'],
    'http_req_failed{scenario:measure}': ['rate==0'],
  },
};

const prefectures = ['', '北海道', '東京都', '千葉県', '大阪府', '沖縄県'];

function dateAfter(days) {
  const d = new Date();
  d.setDate(d.getDate() + days);
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${d.getFullYear()}-${month}-${day}`;
}

function pick(list) {
  return list[Math.floor(Math.random() * list.length)];
}

export default function () {
  const offset = 1 + Math.floor(Math.random() * 90);
  const nights = 1 + Math.floor(Math.random() * 3);
  const guests = 1 + Math.floor(Math.random() * 3);
  const prefecture = pick(prefectures);

  let url = `${baseUrl}/room-types/search`
    + `?checkin=${dateAfter(offset)}`
    + `&checkout=${dateAfter(offset + nights)}`
    + `&guests=${guests}`;
  if (prefecture !== '') {
    url += `&prefecture=${encodeURIComponent(prefecture)}`;
  }

  const res = http.get(url);
  check(res, {
    '200 が返る': (r) => r.status === 200,
  });
}
