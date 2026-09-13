const BASE_URL = import.meta.env.VITE_API_BASE_URL ?? "http://localhost:8080/api/v1";

export class ApiError extends Error {
  constructor(
    readonly code: string,
    readonly status: number,
    message: string,
  ) {
    super(message);
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(`${BASE_URL}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...init?.headers },
  });

  if (!response.ok) {
    const body = await response.json().catch(() => null);
    throw new ApiError(
      body?.code ?? "UNKNOWN",
      response.status,
      body?.message ?? "通信に失敗しました",
    );
  }

  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

export type SearchCriteria = {
  prefecture?: string;
  checkin: string;
  checkout: string;
  guests: number;
  maxFee?: number;
};

export type SearchResult = {
  roomTypeId: string;
  roomTypeName: string;
  capacity: number;
  hasPrivateBath: boolean;
  hasBalcony: boolean;
  accommodationId: string;
  accommodationName: string;
  prefecture: string;
  city: string;
  nights: number;
  totalFee: number;
};

export type RoomType = {
  roomTypeId: string;
  accommodationId: string;
  name: string;
  capacity: number;
  hasPrivateBath: boolean;
  hasBalcony: boolean;
};

export type Guest = { firstName: string; lastName: string };

export type BookingStatus =
  | "temporary_hold"
  | "processing_payment"
  | "confirmed"
  | "cancelled";

export type Booking = {
  bookingId: string;
  bookerId: string;
  roomTypeId: string;
  checkinDate: string;
  checkoutDate: string;
  nights: number;
  totalFee: number;
  status: BookingStatus;
  guests: Guest[];
};

export type BookingSummary = {
  bookingId: string;
  roomTypeId: string;
  checkinDate: string;
  checkoutDate: string;
  totalFee: number;
  cancellationFee: number;
  status: BookingStatus;
};

export type Booker = {
  bookerId: string;
  firstName: string;
  lastName: string;
  phoneNumber: string;
};

export type BookerInput = {
  firstName: string;
  lastName: string;
  phoneNumber: string;
  postalCode: string;
  prefecture: string;
  city: string;
  streetAddress: string;
  building: string;
};

export const statusLabel: Record<BookingStatus, string> = {
  temporary_hold: "仮予約",
  processing_payment: "決済処理中",
  confirmed: "確定",
  cancelled: "キャンセル済み",
};

export const searchRoomTypes = (c: SearchCriteria) => {
  const params = new URLSearchParams({
    checkin: c.checkin,
    checkout: c.checkout,
    guests: String(c.guests),
  });
  if (c.prefecture) params.set("prefecture", c.prefecture);
  if (c.maxFee) params.set("max_fee", String(c.maxFee));
  return request<SearchResult[]>(`/room-types/search?${params}`);
};

export const getRoomType = (roomTypeId: string) =>
  request<RoomType>(`/room-types/${roomTypeId}`);

export const createBooker = (input: BookerInput) =>
  request<Booker>("/bookers", { method: "POST", body: JSON.stringify(input) });

export const listBookings = (bookerId: string) =>
  request<BookingSummary[]>(`/bookers/${bookerId}/bookings`);

export const book = (input: {
  bookerId: string;
  roomTypeId: string;
  checkinDate: string;
  checkoutDate: string;
  guests: Guest[];
}) => request<Booking>("/bookings", { method: "POST", body: JSON.stringify(input) });

export const getBooking = (bookingId: string) =>
  request<Booking>(`/bookings/${bookingId}`);

export const pay = (bookingId: string, result: "success" | "failure" | "timeout") =>
  request<Booking>(`/bookings/${bookingId}/payment`, {
    method: "POST",
    body: JSON.stringify({ result }),
  });

export const cancel = (bookingId: string) =>
  request<{ bookingId: string; status: string; cancellationFee: number }>(
    `/bookings/${bookingId}/cancel`,
    { method: "POST" },
  );
