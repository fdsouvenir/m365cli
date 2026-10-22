# /// script
# requires-python = ">=3.10"
# dependencies = ["msal[broker]>=1.20,<2"]
# ///
"""WAM (Web Account Manager) authentication helper for m365cli.

Invoked as a subprocess by the m365cli CLI on Windows.
Reads a JSON request from stdin, performs WAM-brokered authentication
via MSAL Python, and writes a JSON response to stdout.

Protocol:
  Request:  {"action": "login"|"acquire_silent", "client_id": "...", "authority": "...", "scopes": [...], ...}
  Response: {"access_token": "...", ...} or {"error": "...", "error_description": "..."}
"""

import ctypes
from ctypes import wintypes
import json
import os
import sys
import tempfile
from contextlib import contextmanager

import msal

try:
    import msvcrt
except ImportError:
    msvcrt = None


def error_response(code: str, description: str) -> dict:
    return {"error": code, "error_description": description}


def token_cache_path() -> str:
    base = os.getenv("LOCALAPPDATA") or os.getenv("APPDATA") or os.path.expanduser("~")
    cache_dir = os.path.join(base, "m365cli")
    os.makedirs(cache_dir, exist_ok=True)
    return os.path.join(cache_dir, "wam-token-cache.dpapi")


@contextmanager
def cache_lock(cache_path: str):
    lock_path = f"{cache_path}.lock"
    with open(lock_path, "a+b") as lock_file:
        if msvcrt is not None:
            lock_file.seek(0, os.SEEK_END)
            if lock_file.tell() == 0:
                lock_file.write(b"\0")
                lock_file.flush()
            lock_file.seek(0)
            msvcrt.locking(lock_file.fileno(), msvcrt.LK_LOCK, 1)
        try:
            yield
        finally:
            if msvcrt is not None:
                lock_file.seek(0)
                msvcrt.locking(lock_file.fileno(), msvcrt.LK_UNLCK, 1)


def protect_cache(data: bytes, decrypt: bool = False) -> bytes:
    """Bind persisted broker tokens to the Windows user using DPAPI."""
    if os.name != "nt":
        raise RuntimeError("Broker cache protection requires Windows")

    class DataBlob(ctypes.Structure):
        _fields_ = [("size", wintypes.DWORD), ("data", ctypes.POINTER(ctypes.c_ubyte))]

    crypt32 = ctypes.WinDLL("crypt32", use_last_error=True)
    kernel32 = ctypes.WinDLL("kernel32", use_last_error=True)
    function = crypt32.CryptUnprotectData if decrypt else crypt32.CryptProtectData
    function.argtypes = [ctypes.POINTER(DataBlob), ctypes.c_void_p, ctypes.POINTER(DataBlob), ctypes.c_void_p, ctypes.c_void_p, wintypes.DWORD, ctypes.POINTER(DataBlob)]
    function.restype = wintypes.BOOL
    kernel32.LocalFree.argtypes = [ctypes.c_void_p]
    kernel32.LocalFree.restype = ctypes.c_void_p
    buffer = ctypes.create_string_buffer(data)
    source = DataBlob(len(data), ctypes.cast(buffer, ctypes.POINTER(ctypes.c_ubyte)))
    result = DataBlob()
    if not function(ctypes.byref(source), None, None, None, None, 1, ctypes.byref(result)):
        raise ctypes.WinError(ctypes.get_last_error())
    try:
        return ctypes.string_at(result.data, result.size)
    finally:
        kernel32.LocalFree(result.data)


def load_token_cache(cache: msal.SerializableTokenCache, cache_path: str) -> None:
    if not os.path.exists(cache_path):
        return
    with open(cache_path, "rb") as f:
        state = f.read()
    if state:
        cache.deserialize(protect_cache(state, decrypt=True).decode("utf-8"))


def save_token_cache(cache: msal.SerializableTokenCache, cache_path: str) -> None:
    if not cache.has_state_changed:
        return

    directory = os.path.dirname(cache_path)
    os.makedirs(directory, exist_ok=True)
    fd, temp_path = tempfile.mkstemp(
        dir=directory, prefix="wam-token-cache-", suffix=".dpapi"
    )
    try:
        with os.fdopen(fd, "wb") as f:
            f.write(protect_cache(cache.serialize().encode("utf-8")))
        os.replace(temp_path, cache_path)
    finally:
        if os.path.exists(temp_path):
            os.remove(temp_path)


def login(app: msal.PublicClientApplication, scopes: list[str]) -> dict:
    result = app.acquire_token_interactive(
        scopes=scopes,
        parent_window_handle=app.CONSOLE_WINDOW_HANDLE,
    )
    if "error" in result:
        return error_response(result["error"], result.get("error_description", ""))

    return token_response(result, result.get("account"))


def acquire_silent(
    app: msal.PublicClientApplication,
    scopes: list[str],
    account_id: str,
    username: str,
) -> dict:
    accounts = app.get_accounts()
    account = find_account(accounts, account_id, username)
    if account is None:
        return error_response(
            "no_account",
            "No cached account found. Run `m365cli auth login` to sign in again.",
        )

    result = app.acquire_token_silent(scopes, account=account)
    if result is None:
        return error_response(
            "silent_failed",
            "Silent token acquisition returned no result. Run `m365cli auth login` to sign in again.",
        )
    if "error" in result:
        return error_response(result["error"], result.get("error_description", ""))

    return token_response(result, account)


def find_account(
    accounts: list[dict], account_id: str, username: str
) -> dict | None:
    if account_id:
        for a in accounts:
            if a.get("local_account_id", "").lower() == account_id.lower():
                return a
            # An explicit ID is authoritative; never fall back to a same-name user.
        return None
    if username:
        for a in accounts:
            if a.get("username", "").lower() == username.lower():
                return a
    return None


def token_response(result: dict, account: dict | None = None) -> dict:
    scope = result.get("scope", [])
    if isinstance(scope, list):
        scope = " ".join(scope)

    resp = {
        "access_token": result.get("access_token", ""),
        "token_type": result.get("token_type", "Bearer"),
        "scope": scope,
        "expires_in": result.get("expires_in", 3600),
        "id_token": result.get("id_token", ""),
    }

    if isinstance(account, dict):
        resp["account_id"] = account.get("local_account_id", "")
        resp["username"] = account.get("username", "")
        resp["tenant_id"] = account.get("realm", "")

    claims = result.get("id_token_claims")
    if claims and isinstance(claims, dict):
        resp["id_token_claims"] = {
            "oid": claims.get("oid", ""),
            "sub": claims.get("sub", ""),
            "preferred_username": claims.get("preferred_username", ""),
            "email": claims.get("email", ""),
            "upn": claims.get("upn", ""),
            "tid": claims.get("tid", ""),
        }
        if not resp.get("account_id"):
            resp["account_id"] = claims.get("oid", "") or claims.get("sub", "")
        if not resp.get("username"):
            resp["username"] = (
                claims.get("preferred_username", "")
                or claims.get("email", "")
                or claims.get("upn", "")
            )
        if not resp.get("tenant_id"):
            resp["tenant_id"] = claims.get("tid", "")

    return resp


def main() -> None:
    try:
        request = json.load(sys.stdin)
    except (json.JSONDecodeError, ValueError) as e:
        json.dump(error_response("invalid_request", str(e)), sys.stdout)
        sys.exit(1)

    action = request.get("action", "")
    client_id = request.get("client_id", "")
    authority_value = request.get("authority", "organizations")
    scopes = request.get("scopes", [])
    if isinstance(scopes, str):
        scopes = [s for s in scopes.split(" ") if s]

    # MSAL adds these reserved OIDC scopes automatically and rejects them as
    # explicit requested scopes. Keep the delegated Graph scopes only.
    if isinstance(scopes, list):
        scopes = [scope for scope in scopes if isinstance(scope, str) and scope.lower() not in {"openid", "profile", "offline_access"}]

    if not client_id:
        json.dump(error_response("missing_client_id", "client_id is required"), sys.stdout)
        sys.exit(1)
    if not isinstance(scopes, list) or not scopes:
        json.dump(
            error_response("missing_scopes", "scopes must be a non-empty list"),
            sys.stdout,
        )
        sys.exit(1)

    authority = f"https://login.microsoftonline.com/{authority_value}"

    cache = msal.SerializableTokenCache()
    cache_path = token_cache_path()
    with cache_lock(cache_path):
        load_token_cache(cache, cache_path)
        app = msal.PublicClientApplication(
            client_id,
            authority=authority,
            token_cache=cache,
            enable_broker_on_windows=True,
        )

        if action == "login":
            result = login(app, scopes)
        elif action == "acquire_silent":
            account_id = request.get("account_id", "")
            username = request.get("username", "")
            if not account_id and not username:
                result = error_response(
                    "missing_account",
                    "acquire_silent requires account_id or username",
                )
            else:
                result = acquire_silent(app, scopes, account_id, username)
        else:
            result = error_response("unknown_action", f"Unknown action: {action}")

        save_token_cache(cache, cache_path)

    json.dump(result, sys.stdout)
    if "error" in result:
        sys.exit(1)


if __name__ == "__main__":
    main()
