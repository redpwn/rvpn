import { Dispatch, SetStateAction, useEffect, useState } from "react";
import { GetControlPlaneAuth, Status } from "../../../wailsjs/go/main/App";

import logo from "../../assets/images/logo_w.svg";
import Button from "../../components/Button";
import { darkToast, ToastType } from "../../util";

import "./style.css";

interface HomeProps {
  setAuth: Dispatch<SetStateAction<string>>;
}

interface StatusMessageProps {
  status: string;
}

const StatusMessage = (props: StatusMessageProps) => {
  switch (props.status) {
    case "connected":
      return (
        <div>
          <span>rVPN is currently </span>
          <span className="text-green">connected</span>
          <span> to a profile!</span>
        </div>
      );
    case "disconnected":
      return (
        <div>
          <span>rVPN is currently </span>
          <span className="text-red">disconnected!</span>
          <span> select a profile below to connect!</span>
        </div>
      );
    default:
      return (
        <div>
          <span>rVPN status is unknown </span>
        </div>
      );
  }
};

const Home = (props: HomeProps) => {
  const [loading, setLoading] = useState(true);
  const [authToken, setAuthToken] = useState("");
  const [rVPNStatus, setRVPNStatus] = useState("");

  const fetchData = async () => {
    const authTokenResp = await GetControlPlaneAuth();
    if (authTokenResp.success) {
      setAuthToken(authTokenResp.data);
    } else {
      darkToast(ToastType.Error, "unable to get auth token");
      console.error(authTokenResp.error);
      setLoading(false);
      return;
    }

    const vpnStatusResp = await Status();
    if (vpnStatusResp.success) {
      switch (vpnStatusResp.data) {
        case "rVPN is currently connected to a profile":
          setRVPNStatus("connected");
          break;
        case "rVPN is not currently connected to a profile":
          setRVPNStatus("disconnected");
          break;
        default:
          setRVPNStatus("");
          break;
      }
    } else {
      darkToast(ToastType.Error, "unable to get vpn status");
      console.error(vpnStatusResp.error);
      setLoading(false);
      return;
    }

    setLoading(false);
  };

  useEffect(() => {
    fetchData().catch(console.error);
  }, []);

  const handleLogout = () => {
    darkToast(ToastType.Success, "clicked log out");
  };

  const handleRefresh = () => {
    darkToast(ToastType.Blank, "refreshing status...");
    fetchData().catch(console.error);
  };

  return (
    <div className="home-container">
      <div className="home-header">
        <div>
          <img src={logo} />
        </div>
        <div className="header-right">
          <span>asdfasdf</span>
          <Button text="Logout" onClick={handleLogout} />
        </div>
      </div>
      <div className="home-body">
        <div className="body-section">
          <div className="status-header">
            <div className="status-text">Status</div>
            <div className="status-refresh">
              <Button text="refresh" onClick={handleRefresh} />
            </div>
          </div>
          <div className="status-body">
            <StatusMessage status={rVPNStatus} />
          </div>
        </div>
      </div>
    </div>
  );
};

export default Home;
