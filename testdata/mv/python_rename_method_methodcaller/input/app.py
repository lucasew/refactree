from operator import methodcaller
import operator


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_methodcaller():
    return methodcaller("run")(A()) + methodcaller("run")(B())


def use_operator_methodcaller():
    return operator.methodcaller("run")(A()) + operator.methodcaller("run")(B())


def use_preserves_b():
    return methodcaller("run")(B())
