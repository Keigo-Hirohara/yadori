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

export type Accommodation = {
  accommodationId: string;
  name: string;
  phoneNumber: string;
  postalCode: string;
  prefecture: string;
  city: string;
  streetAddress: string;
  building: string;
};

export type AccommodationInput = Omit<Accommodation, "accommodationId">;

export type RoomType = {
  roomTypeId: string;
  accommodationId: string;
  name: string;
  capacity: number;
  hasPrivateBath: boolean;
  hasBalcony: boolean;
};

export type RoomTypeInput = {
  name: string;
  capacity: number;
  hasPrivateBath: boolean;
  hasBalcony: boolean;
};

export type Inventory = {
  date: string;
  fee: number;
  quantityAvailable: number;
  heldCount: number;
  available: number;
  isClosed: boolean;
};

export const listAccommodations = () => request<Accommodation[]>("/admin/accommodations");

export const getAccommodation = (id: string) =>
  request<Accommodation>(`/admin/accommodations/${id}`);

export const createAccommodation = (input: AccommodationInput) =>
  request<Accommodation>("/admin/accommodations", {
    method: "POST",
    body: JSON.stringify(input),
  });

export const listRoomTypes = (accommodationId: string) =>
  request<RoomType[]>(`/admin/accommodations/${accommodationId}/room-types`);

export const getRoomType = (roomTypeId: string) =>
  request<RoomType>(`/admin/room-types/${roomTypeId}`);

export const createRoomType = (accommodationId: string, input: RoomTypeInput) =>
  request<RoomType>(`/admin/accommodations/${accommodationId}/room-types`, {
    method: "POST",
    body: JSON.stringify(input),
  });

export const listInventories = (roomTypeId: string, from: string, to: string) =>
  request<Inventory[]>(
    `/admin/room-types/${roomTypeId}/inventories?from=${from}&to=${to}`,
  );

export const registerInventory = (
  roomTypeId: string,
  input: { date: string; quantity: number; fee: number },
) =>
  request<void>(`/admin/room-types/${roomTypeId}/inventories`, {
    method: "POST",
    body: JSON.stringify(input),
  });

export const changeQuantity = (roomTypeId: string, date: string, quantity: number) =>
  request<void>(`/admin/room-types/${roomTypeId}/inventories/${date}/quantity`, {
    method: "PUT",
    body: JSON.stringify({ quantity }),
  });

export const changeFee = (roomTypeId: string, date: string, fee: number) =>
  request<void>(`/admin/room-types/${roomTypeId}/inventories/${date}/fee`, {
    method: "PUT",
    body: JSON.stringify({ fee }),
  });

export const closeInventory = (roomTypeId: string, date: string) =>
  request<void>(`/admin/room-types/${roomTypeId}/inventories/${date}/close`, {
    method: "POST",
  });

export const reopenInventory = (roomTypeId: string, date: string) =>
  request<void>(`/admin/room-types/${roomTypeId}/inventories/${date}/reopen`, {
    method: "POST",
  });
