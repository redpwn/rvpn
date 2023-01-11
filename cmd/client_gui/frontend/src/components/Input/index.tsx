import "./style.css";

interface InputProps {
  value: string;
  onChange(e: React.ChangeEvent<HTMLInputElement>): any;

  //optional styling props
  placeholder?: string;
}

const Input = (props: InputProps) => {
  return (
    <input
      value={props.value}
      onChange={props.onChange}
      placeholder={props.placeholder}
      className="styled-input"
    />
  );
};

export default Input;
