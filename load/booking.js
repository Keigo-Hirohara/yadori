import http from 'k6/http';
import { check } from 'k6';
import exec from 'k6/execution';
import { Counter } from 'k6/metrics';

const baseUrl = __ENV.BASE_URL || 'http://localhost:8080/api/v1';
const authOrigin = __ENV.AUTH_ORIGIN || 'http://localhost:8180';
const clientSecret = __ENV.LOADTEST_CLIENT_SECRET || 'loadtest';
const startDate = __ENV.START_DATE;
const roomTypeIds = (__ENV.ROOM_TYPE_IDS || '').split(',').filter((id) => id !== '');
const days = Number(__ENV.DAYS || 72);

const searchRate = 70;
const bookingRate = 1;
const blockDays = 3;
const measureOffset = 50;

const unexpected = new Counter('unexpected');

export const options = {
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(95)', 'p(99)'],
  scenarios: {
    search_warmup: {
      executor: 'constant-arrival-rate',
      exec: 'search',
      rate: searchRate,
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 50,
      maxVUs: 200,
    },
    search_measure: {
      executor: 'constant-arrival-rate',
      exec: 'search',
      rate: searchRate,
      timeUnit: '1s',
      duration: '3m',
      startTime: '30s',
      preAllocatedVUs: 50,
      maxVUs: 200,
    },
    booking_warmup: {
      executor: 'constant-arrival-rate',
      exec: 'book',
      rate: bookingRate,
      timeUnit: '1s',
      duration: '30s',
      preAllocatedVUs: 5,
      maxVUs: 20,
    },
    booking_measure: {
      executor: 'constant-arrival-rate',
      exec: 'book',
      rate: bookingRate,
      timeUnit: '1s',
      duration: '3m',
      startTime: '30s',
      preAllocatedVUs: 5,
      maxVUs: 20,
    },
  },
  thresholds: {
    'http_req_duration{scenario:search_measure}': ['p(95)<150', 'p(99)<300'],
    'http_req_failed{scenario:search_measure}': ['rate==0'],
    'http_req_duration{scenario:booking_measure,name:book}': ['p(95)<500', 'p(99)<1000'],
    'http_req_duration{scenario:booking_measure,name:payment}': ['p(95)<500', 'p(99)<1000'],
    unexpected: ['count==0'],
  },
};

const prefectures = ['', '北海道', '東京都', '千葉県', '大阪府', '沖縄県'];

function addDays(date, n) {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

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

export function setup() {
  if (!startDate || roomTypeIds.length === 0) {
    throw new Error('START_DATE と ROOM_TYPE_IDS を指定してください');
  }

  const tokenRes = http.post(`${authOrigin}/realms/yadori-booker/protocol/openid-connect/token`, {
    grant_type: 'client_credentials',
    client_id: 'yadori-loadtest',
    client_secret: clientSecret,
  });
  if (tokenRes.status !== 200) {
    throw new Error(`トークンを取得できません: ${tokenRes.status}`);
  }
  const token = tokenRes.json('access_token');

  const registerRes = http.post(`${baseUrl}/bookers/me`, JSON.stringify({
    firstName: '試験',
    lastName: '分散',
    phoneNumber: '09012345678',
    postalCode: '1000001',
    prefecture: '東京都',
    city: '千代田区',
    streetAddress: '1-1-1',
  }), {
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
  });
  if (registerRes.status !== 201 && registerRes.status !== 409) {
    throw new Error(`会員登録に失敗しました: ${registerRes.status}`);
  }

  return { token };
}

export function search() {
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
    '検索: 200 が返る': (r) => r.status === 200,
  });
}

export function book(data) {
  unexpected.add(0);

  let n = exec.scenario.iterationInTest;
  if (exec.scenario.name === 'booking_measure') {
    n += measureOffset;
  }
  const blocksPerRoomType = Math.floor(days / blockDays);
  if (n >= roomTypeIds.length * blocksPerRoomType) {
    exec.test.abort(`区画が足りません（${n} 番目）`);
  }

  const roomTypeId = roomTypeIds[n % roomTypeIds.length];
  const block = Math.floor(n / roomTypeIds.length);
  const nights = 1 + (n % blockDays);
  const checkin = addDays(startDate, block * blockDays);
  const checkout = addDays(checkin, nights);
  const headers = { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` };

  const searchRes = http.get(
    `${baseUrl}/room-types/search?checkin=${checkin}&checkout=${checkout}&guests=1`,
    { tags: { name: 'flow_search' } },
  );
  const found = searchRes.status === 200
    && searchRes.json().some((r) => r.roomTypeId === roomTypeId);
  if (!check(found, { '予約: 割り当ての部屋タイプが検索に出る': (f) => f })) {
    unexpected.add(1);
    return;
  }

  const bookRes = http.post(`${baseUrl}/bookings`, JSON.stringify({
    roomTypeId,
    checkinDate: checkin,
    checkoutDate: checkout,
    guests: [{ firstName: '試験', lastName: '宿泊者' }],
  }), { headers, tags: { name: 'book' } });
  if (!check(bookRes, { '予約: 201 が返る': (r) => r.status === 201 })) {
    unexpected.add(1);
    return;
  }

  const bookingId = bookRes.json('bookingId');
  const payRes = http.post(`${baseUrl}/bookings/${bookingId}/payment`, JSON.stringify({
    result: 'success',
  }), { headers, tags: { name: 'payment' } });
  if (!check(payRes, { '確定: 200 が返る': (r) => r.status === 200 })) {
    unexpected.add(1);
  }
}
