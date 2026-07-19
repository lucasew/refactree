from operator import methodcaller
import operator


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_methodcaller():
    return methodcaller("execute")(A()) + methodcaller("run")(B())


def use_operator_methodcaller():
    return operator.methodcaller("execute")(A()) + operator.methodcaller("run")(B())


def use_preserves_b():
    return methodcaller("run")(B())
