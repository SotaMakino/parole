let login = async (~username, ~password) =>
  switch await ApiClient.request(
    "/login",
    ~method_="POST",
    ~body={"username": username, "password": password},
  ) {
  | Ok(_) => Ok()
  | Error(e) => Error(e)
  }

let signup = async (~username, ~password) =>
  switch await ApiClient.request(
    "/signup",
    ~method_="POST",
    ~body={"username": username, "password": password},
  ) {
  | Ok(_) => await login(~username, ~password)
  | Error(e) => Error(e)
  }

let logout = async () =>
  switch await ApiClient.request("/logout", ~method_="POST") {
  | Ok(_) => Ok()
  | Error(e) => Error(e)
  }
