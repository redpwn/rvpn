import { Dispatch, SetStateAction, useEffect, useState } from "react";
import {
  Connect,
  Disconnect,
  GetControlPlaneAuth,
  ListTargets,
  Status,
} from "../../../wailsjs/go/main/App";
import { common } from "../../../wailsjs/go/models";

import logo from "../../assets/images/logo_w.svg";
import Button from "../../components/Button";
import Input from "../../components/Input";
import { darkToast, ToastType } from "../../util";

import "./style.css";

interface HomeProps {
  setAuth: Dispatch<SetStateAction<string>>;
}

interface TargetInfo {
  name: string;
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
  const [targetList, setTargetList] = useState(new Array<TargetInfo>());

  const [searchInput, setSearchInput] = useState("");
  const [displayTargetList, setDisplayTargetList] = useState(
    new Array<TargetInfo>()
  );

  const filterTargetList = (search: string) => {
    // helper to filter the target list
    if (search == "") {
      // no search query, display all targets
      setDisplayTargetList(targetList);
    } else {
      // there was a specific search query
      const filteredList: TargetInfo[] = [];
      for (let i = 0; i < targetList.length; i++) {
        if (targetList[i].name.includes(search)) {
          filteredList.push(targetList[i]);
        }
      }

      setDisplayTargetList(filteredList);
    }
  };

  const fetchStatus = async () => {
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
      return;
    }
  };

  const fetchTargets = async () => {
    const listTargetsResp = await ListTargets();
    if (listTargetsResp.success) {
      // data is JSON array of targets the user has access to
      const targetList: TargetInfo[] = JSON.parse(listTargetsResp.data);

      setTargetList(targetList);
      setDisplayTargetList(targetList);
    } else {
      darkToast(ToastType.Error, "unable to get possible targets");
      console.error(listTargetsResp.error);
      return;
    }
  };

  const fetchAllData = async () => {
    const authTokenResp = await GetControlPlaneAuth();
    if (authTokenResp.success) {
      setAuthToken(authTokenResp.data);
    } else {
      darkToast(ToastType.Error, "unable to get auth token");
      console.error(authTokenResp.error);
      setLoading(false);
      return;
    }

    // if getting auth token succeeds then continue fetching
    await fetchStatus();
    await fetchTargets();

    setLoading(false);
  };

  useEffect(() => {
    // on initial load, fetch all data
    fetchAllData().catch(console.error);
  }, []);

  const handleLogout = () => {
    darkToast(ToastType.Success, "clicked log out");
    // TODO: logout behavior
  };

  const handleStatusRefresh = () => {
    darkToast(ToastType.Blank, "refreshing status...");
    fetchStatus().catch(console.error);
  };

  const handleProfileRefresh = () => {
    darkToast(ToastType.Blank, "refreshing targets...");
    fetchTargets().catch(console.error);
  };

  const handleProfileConnect = (profile: string) => {
    // this is a core handler which instructs the rVPN daemon to connect
    const buttonHandler = async () => {
      // connect to a profile and refresh status after connection finishes
      darkToast(ToastType.Blank, "connecting to " + profile);

      // issue connect command to rVPN daemon
      const opts: common.ClientOptions = {
        subnets: ["0.0.0.0/0"],
      };
      const connectResp = await Connect(profile, opts);
      if (connectResp.success) {
        // rVPN successfully connected to target server
        darkToast(ToastType.Success, "connected to " + profile);
        setRVPNStatus("connected");
      } else {
        // rVPN failed to connect to target server
        darkToast(ToastType.Error, "failed to connect to " + profile);
        console.error(connectResp.error);
      }
    };

    return buttonHandler;
  };

  const handleProfileDisconnect = async () => {
    // this is a core handler which instructs the rVPN daemon to disconnect
    const disconnectResp = await Disconnect();
    if (disconnectResp.success) {
      // rVPN was successfully disconnected
      darkToast(ToastType.Success, "successfully disconnected");
      setRVPNStatus("disconnected");
    } else {
      // rVPN failed to disconnect
      darkToast(ToastType.Error, "failed to disconnect");
      console.error(disconnectResp.error);
    }
  };

  const handleSearchInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setSearchInput(e.target.value);
    filterTargetList(e.target.value);
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
            <div className="status-control">
              <Button text="refresh" onClick={handleStatusRefresh} />
              {rVPNStatus === "connected" && (
                <Button text="disconnect" onClick={handleProfileDisconnect} />
              )}
            </div>
          </div>
          <div className="status-body">
            <StatusMessage status={rVPNStatus} />
          </div>
        </div>
        <div className="body-section">
          <div className="profiles-header">
            <div className="profiles-text">Profiles</div>
            <div className="profiles-refresh">
              <Button text="refresh" onClick={handleProfileRefresh} />
            </div>
          </div>
          <div className="profiles-search">
            <form>
              <div className="search-input">
                <Input
                  value={searchInput}
                  onChange={handleSearchInputChange}
                  placeholder="search for a profile"
                />
              </div>
            </form>
          </div>
          <div className="profiles-body">
            {displayTargetList.map((item, i) => {
              return (
                <div key={i} className="profile-row">
                  <div className="profile-info">
                    <span>{item.name}</span>
                    <div className="profile-connect-button">
                      <Button
                        text="connect"
                        onClick={handleProfileConnect(item.name)}
                        disabled={rVPNStatus !== "disconnected"}
                      />
                    </div>
                  </div>
                  <div className="divider"></div>
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
};

export default Home;
