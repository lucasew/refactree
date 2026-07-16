class A(Exception):
    def execute(self):
        return 1


class B(Exception):
    def run(self):
        return 2


def use_except_star():
    try:
        raise ExceptionGroup("eg", [A()])
    except* A as e:
        for a in e.exceptions:
            a.execute()
    except* B as f:
        for b in f.exceptions:
            b.run()


def use_except_star_comp():
    try:
        raise ExceptionGroup("eg", [A(), B()])
    except* A as e:
        return [a.execute() for a in e.exceptions]
    except* B as f:
        return [b.run() for b in f.exceptions]
