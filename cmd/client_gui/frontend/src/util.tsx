import toast from "react-hot-toast";

export enum ToastType {
  Success = 1,
  Error,
  Blank,
}

export const darkToast = (toastType: ToastType, msg: string) => {
  const darkToastStyle = {
    borderRadius: "10px",
    background: "#333",
    color: "#fff",
  };

  switch (toastType) {
    case ToastType.Success:
      toast.success(msg, { style: darkToastStyle });
      break;
    case ToastType.Error:
      toast.error(msg, { style: darkToastStyle });
      break;
    case ToastType.Blank:
      toast(msg, { style: darkToastStyle });
      break;
    default:
      toast(msg, { style: darkToastStyle });
      break;
  }
};
