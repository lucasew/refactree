class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_match(x):
    match x:
        case A() as a:
            return a.execute()
        case B() as b:
            return b.run()


def use_with():
    with A() as a:
        return a.execute()


def use_with_b():
    with B() as b:
        return b.run()


def use_except():
    try:
        raise A()
    except A as e:
        return e.execute()
    except B as f:
        return f.run()
