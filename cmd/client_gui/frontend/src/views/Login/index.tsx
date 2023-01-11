import { Dispatch, SetStateAction, useState } from "react";
import logo from "../../assets/images/logo_w.svg";
import Button from "../../components/Button";
import Footer from "../../components/Footer";
import Input from "../../components/Input";

import { Login as LoginRVPN } from "../../../wailsjs/go/main/App";

import "./style.css";
import { darkToast, ToastType } from "../../util";

interface LoginProps {
  setAuth: Dispatch<SetStateAction<string>>;
}

const Login = (props: LoginProps) => {
  const [authToken, setAuthToken] = useState("");

  const handleTokenInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setAuthToken(e.target.value);
  };

  const handleFormSubmit = async () => {
    const loginResp = await LoginRVPN(authToken);

    if (loginResp.success) {
      props.setAuth(authToken);
    } else {
      // something went wrong
      darkToast(ToastType.Error, "failed to login");
      console.error(loginResp.error);
    }
  };

  return (
    <div>
      <div className="login-container">
        <img src={logo} />
        <div className="signin-text">Sign In</div>
        <span className="descriptor-text">
          retrieve your login token from{" "}
          <a href="https://rvpn.dev" className="descriptor-link">
            https://rvpn.dev
          </a>
        </span>
        <form>
          <div className="authtoken-input">
            <Input
              value={authToken}
              onChange={handleTokenInputChange}
              placeholder="login token"
            />
          </div>
          <div className="submit-container">
            <div className="submit-button">
              <Button onClick={handleFormSubmit} text="Login" />
            </div>
          </div>
        </form>
      </div>
      <div className="login-footer">
        <Footer />
      </div>
    </div>
  );
};

export default Login;
