public @interface Ann {
  String helper() default "h";

  String stay() default "s";
}

@Ann(helper = "x", stay = "y")
class Main {}
