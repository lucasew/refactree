public @interface Ann {
  String assist() default "h";

  String stay() default "s";
}

@Ann(assist = "x", stay = "y")
class Main {}
