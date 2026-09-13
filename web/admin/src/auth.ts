import { UserManager, WebStorageStateStore, type User } from "oidc-client-ts";

const authority = import.meta.env.VITE_OIDC_AUTHORITY ?? "http://localhost:8180/realms/yadori-operator";
const clientId = import.meta.env.VITE_OIDC_CLIENT_ID ?? "yadori-admin-web";

export const userManager = new UserManager({
  authority,
  client_id: clientId,
  redirect_uri: `${window.location.origin}/callback`,
  post_logout_redirect_uri: window.location.origin,
  response_type: "code",
  scope: "openid profile email",
  automaticSilentRenew: true,
  userStore: new WebStorageStateStore({ store: window.localStorage }),
});

export const getUser = (): Promise<User | null> => userManager.getUser();

export const login = (returnTo: string = window.location.pathname + window.location.search) =>
  userManager.signinRedirect({ state: returnTo });

export const logout = () => userManager.signoutRedirect();

export const completeLogin = async (): Promise<string> => {
  const user = await userManager.signinCallback();
  const returnTo = typeof user?.state === "string" ? user.state : "/";
  return returnTo;
};

export const getAccessToken = async (): Promise<string | null> => {
  const user = await userManager.getUser();
  if (!user || user.expired) return null;
  return user.access_token;
};
