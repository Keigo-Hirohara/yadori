import http from 'k6/http';
import { check } from 'k6';
import { Counter } from 'k6/metrics';

const baseUrl = __ENV.BASE_URL || 'http://localhost:8080/api/v1';
const authOrigin = __ENV.AUTH_ORIGIN || 'http://localhost:8180';
const clientSecret = __ENV.LOADTEST_CLIENT_SECRET || 'loadtest';
const roomTypeId = __ENV.ROOM_TYPE_ID;
const checkin = __ENV.CHECKIN;

const booked = new Counter('booked');
const soldOut = new Counter('sold_out');
const busy = new Counter('busy');
const unexpected = new Counter('unexpected');

export const options = {
  summaryTrendStats: ['avg', 'min', 'med', 'max', 'p(95)', 'p(99)'],
  scenarios: {
    rush: {
      executor: 'per-vu-iterations',
      vus: 100,
      iterations: 1,
    },
  },
  thresholds: {
    booked: ['count==3'],
    unexpected: ['count==0'],
    'http_req_duration{name:book}': ['max<3000'],
  },
};

function nextDay(date) {
  const d = new Date(`${date}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + 1);
  return d.toISOString().slice(0, 10);
}

export function setup() {
  if (!roomTypeId || !checkin) {
    throw new Error('ROOM_TYPE_ID と CHECKIN を指定してください');
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
    lastName: '集中',
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

export default function (data) {
  const res = http.post(`${baseUrl}/bookings`, JSON.stringify({
    roomTypeId,
    checkinDate: checkin,
    checkoutDate: nextDay(checkin),
    guests: [{ firstName: '試験', lastName: '宿泊者' }],
  }), {
    headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${data.token}` },
    tags: { name: 'book' },
  });

  booked.add(res.status === 201 ? 1 : 0);
  soldOut.add(res.status === 409 ? 1 : 0);
  busy.add(res.status === 503 ? 1 : 0);
  unexpected.add([201, 409, 503].includes(res.status) ? 0 : 1);

  check(res, {
    '201 / 409 / 503 のどれかが返る': (r) => [201, 409, 503].includes(r.status),
  });
}
