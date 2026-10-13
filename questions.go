package processing

import (
	vocab "github.com/go-ap/activitypub"
	"github.com/go-ap/errors"
)

// QuestionActivity processes matching activities
//
// https://www.w3.org/TR/activitystreams-vocabulary/#h-motivations-questions
//
// The Questions use case primarily deals with representing inquiries of any type.
// See 5.4 Representing Questions for more information: https://www.w3.org/TR/activitystreams-vocabulary/#questions
func (p *P) QuestionActivity(q *vocab.Question) (*vocab.Question, error) {
	// NOTE(marius): this behaviour is not in accordance to the spec.
	// Saving the question items as individual objects makes sense to me,
	// but it's not mention in either of the documents we're basing this implementation on.
	// I can't think of any reason why saving them might be a _bad_ idea, so for the moment I'll leave it like this.
	if q.AnyOf != nil {
		return q, vocab.OnItemCollection(q.AnyOf, p.saveQuestionAnswers(q))
	}
	if q.OneOf != nil {
		return q, vocab.OnItemCollection(q.OneOf, p.saveQuestionAnswers(q))
	}
	return q, nil
}

func (p *P) saveQuestionAnswers(q *vocab.Question) func(col *vocab.ItemCollection) error {
	return func(col *vocab.ItemCollection) error {
		errs := make([]error, 0, col.Count())
		for _, ans := range col.Collection() {
			var err error
			if iri := ans.GetLink(); len(iri) == 0 {
				if err = SetIDIfMissing(ans, q, p.createIDFn); err != nil {
					errs = append(errs, err)
					continue
				}
			}

			err = vocab.OnActivity(q, func(act *vocab.Activity) error {
				return p.updateCreateActivityObject(ans, act)
			})
			if err != nil {
				errs = append(errs, err)
				continue
			}

			ans, err = p.s.Save(ans)
			if err != nil {
				errs = append(errs, err)
			}
		}
		return errors.Join(errs...)
	}
}
